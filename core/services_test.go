package core_test

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/engine/buildkit"
	bkgw "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestServicesStartHappy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
		ClientID: "fake-client",
	})

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	svc1 := newStartable("fake-1")
	svc2 := newStartable("fake-2")

	startOne := func(t *testing.T, stub *fakeStartable) {
		_, err := services.Get(ctx, stub)
		require.Error(t, err)

		expected := stub.Succeed()

		running, err := services.Start(ctx, stub)
		require.NoError(t, err)
		require.Equal(t, expected, running)

		running, err = services.Get(ctx, stub)
		require.NoError(t, err)
		require.Equal(t, expected, running)
	}

	t.Run("start one", func(t *testing.T) {
		startOne(t, svc1)
	})

	t.Run("start another", func(t *testing.T) {
		startOne(t, svc2)
	})
}

func TestServicesStartHappyDifferentClients(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	svc := newStartable("fake")

	startOne := func(t *testing.T, stub *fakeStartable, clientID string) {
		ctx := engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
			ClientID: clientID,
		})

		expected := stub.Succeed()

		_, err := services.Get(ctx, stub)
		require.Error(t, err)

		running, err := services.Start(ctx, stub)
		require.NoError(t, err)
		require.Equal(t, expected, running)

		running, err = services.Get(ctx, stub)
		require.NoError(t, err)
		require.Equal(t, expected, running)
	}

	t.Run("start one", func(t *testing.T) {
		startOne(t, svc, "client-1")
	})

	t.Run("start another", func(t *testing.T) {
		startOne(t, svc, "client-2")
	})
}

func TestServicesStartSad(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
		ClientID: "fake-client",
	})

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	stub := newStartable("fake")

	expected := stub.Fail()

	_, err := services.Start(ctx, stub)
	require.Equal(t, expected, err)

	_, err = services.Get(ctx, stub)
	require.Error(t, err)
}

func TestServicesStartConcurrentHappy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
		ClientID: "fake-client",
	})

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	stub := newStartable("fake")

	eg := new(errgroup.Group)
	eg.Go(func() error {
		_, err := services.Start(ctx, stub)
		return err
	})

	// wait for start attempt [hopefully not flaky]
	require.Eventually(t, func() bool {
		return stub.Starts() > 0
	}, 10*time.Second, 10*time.Millisecond)

	// start another attempt
	eg.Go(func() error {
		_, err := services.Start(ctx, stub)
		return err
	})

	// [try to] wait for second start attempt to start waiting
	time.Sleep(100 * time.Millisecond)
	runtime.Gosched()

	// make sure we didn't try to start twice
	require.Equal(t, 1, stub.Starts())

	// allow the first attempt to succeed
	stub.Succeed()

	// make sure all start attempts succeeded
	require.NoError(t, eg.Wait())

	// make sure we didn't try to start twice
	require.Equal(t, 1, stub.Starts())
}

func TestServicesStartConcurrentSad(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
		ClientID: "fake-client",
	})

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	stub := newStartable("fake")

	errs := make(chan error, 100)
	go func() {
		_, err := services.Start(ctx, stub)
		errs <- err
	}()

	// wait for start attempt [hopefully not flaky]
	require.Eventually(t, func() bool {
		return stub.Starts() == 1
	}, 10*time.Second, 10*time.Millisecond)

	// start another attempt
	go func() {
		_, err := services.Start(ctx, stub)
		errs <- err
	}()

	// [try to] wait for second start attempt to start waiting
	time.Sleep(100 * time.Millisecond)
	runtime.Gosched()

	// make sure we didn't try to start twice
	require.Equal(t, 1, stub.Starts())

	// make the first attempt fail
	require.Equal(t, stub.Fail(), <-errs)

	// wait for second start attempt [hopefully not flaky]
	require.Eventually(t, func() bool {
		return stub.Starts() == 2
	}, 10*time.Second, 10*time.Millisecond)

	// make the second attempt fail too
	require.Equal(t, stub.Fail(), <-errs)

	// make sure we didn't try to start more than twice
	require.Equal(t, 2, stub.Starts())

	// make sure Get doesn't wait for any attempts, as they've all failed
	_, err := services.Get(ctx, stub)
	require.Error(t, err)
}

func TestServicesStartConcurrentSadThenHappy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = engine.ContextWithClientMetadata(ctx, &engine.ClientMetadata{
		ClientID: "fake-client",
	})

	stubClient := new(buildkit.Client)
	services := core.NewServices(stubClient)

	stub := newStartable("fake")

	errs := make(chan error, 100)
	go func() {
		_, err := services.Start(ctx, stub)
		errs <- err
	}()

	// wait for start attempt [hopefully not flaky]
	require.Eventually(t, func() bool {
		return stub.Starts() == 1
	}, 10*time.Second, 10*time.Millisecond)

	// start a few more attempts
	for i := 0; i < 3; i++ {
		go func() {
			_, err := services.Start(ctx, stub)
			errs <- err
		}()
	}

	// [try to] wait for second start attempt to start waiting
	time.Sleep(100 * time.Millisecond)
	runtime.Gosched()

	// make sure we didn't try to start twice
	require.Equal(t, 1, stub.Starts())

	// make the first attempt fail
	require.Equal(t, stub.Fail(), <-errs)

	// wait for second start attempt [hopefully not flaky]
	require.Eventually(t, func() bool {
		return stub.Starts() == 2
	}, 10*time.Second, 10*time.Millisecond)

	// make the second attempt succeed
	stub.Succeed()

	// wait for all attempts to succeed
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)

	// make sure we didn't try to start more than twice
	require.Equal(t, 2, stub.Starts())
}

type fakeStartable struct {
	id     string
	digest digest.Digest

	starts       int32 // total start attempts
	startResults chan startResult
}

type startResult struct {
	Started *core.RunningService
	Failed  error
}

func newStartable(id string) *fakeStartable {
	return &fakeStartable{
		id:     id,
		digest: digest.FromString(id),

		// just buffer 100 to keep things simple
		startResults: make(chan startResult, 100),
	}
}

func (f *fakeStartable) Digest() (digest.Digest, error) {
	return f.digest, nil
}

func (f *fakeStartable) Start(context.Context, *buildkit.Client, *core.Services, bool, func(io.Writer, bkgw.ContainerProcess), func(io.Reader), func(io.Reader)) (*core.RunningService, error) {
	atomic.AddInt32(&f.starts, 1)
	res := <-f.startResults
	return res.Started, res.Failed
}

func (f *fakeStartable) Starts() int {
	return int(atomic.LoadInt32(&f.starts))
}

func (f *fakeStartable) Succeed() *core.RunningService {
	running := &core.RunningService{
		Key: core.ServiceKey{
			Digest:   f.digest,
			ClientID: "doesnt-matter",
		},
		Host: f.id + "-host",
	}

	f.startResults <- startResult{
		Started: running,
	}

	return running
}

func (f *fakeStartable) Fail() error {
	err := errors.New("oh no")
	f.startResults <- startResult{
		Failed: err,
	}
	return err
}
