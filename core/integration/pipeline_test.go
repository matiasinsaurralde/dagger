package core

import (
	"fmt"
	"testing"
	"time"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
)

func TestPipeline(t *testing.T) {
	t.Parallel()

	cacheBuster := fmt.Sprintf("%d", time.Now().UTC().UnixNano())

	t.Run("container pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, ctx := connect(t, dagger.WithLogOutput(&logs))

		_, err := c.
			Container().
			Pipeline("container pipeline").
			From(alpineImage).
			WithExec([]string{"echo", cacheBuster}).
			Sync(ctx)

		require.NoError(t, err)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "container pipeline")
	})

	t.Run("directory pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, ctx := connect(t, dagger.WithLogOutput(&logs))

		contents, err := c.
			Directory().
			Pipeline("directory pipeline").
			WithNewFile("/foo", cacheBuster).
			File("/foo").
			Contents(ctx)

		require.NoError(t, err)
		require.Equal(t, contents, cacheBuster)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "directory pipeline")
	})

	t.Run("service pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, ctx := connect(t, dagger.WithLogOutput(&logs))

		srv, url := httpService(ctx, t, c, "Hello, world!")

		hostname, err := srv.Hostname(ctx)
		require.NoError(t, err)

		client := c.Container().
			From(alpineImage).
			WithServiceBinding("www", srv).
			WithExec([]string{"apk", "add", "curl"}).
			WithExec([]string{"curl", "-v", url})

		_, err = client.Sync(ctx)
		require.NoError(t, err)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "service "+hostname)
		require.Regexp(t, `start python -m http.server.*DONE`, logs.String())
	})
}

func TestInternalVertexes(t *testing.T) {
	t.Parallel()

	cacheBuster := fmt.Sprintf("%d", time.Now().UTC().UnixNano())

	t.Run("merge pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, ctx := connect(t, dagger.WithLogOutput(&logs))

		dirA := c.Directory().WithNewFile("/foo", "foo")
		dirB := c.Directory().WithNewFile("/bar", "bar")

		_, err := c.
			Container().
			From(alpineImage).
			WithDirectory("/foo", dirA).
			WithDirectory("/bar", dirB).
			WithExec([]string{"echo", cacheBuster}).
			Sync(ctx)

		require.NoError(t, err)

		require.NoError(t, c.Close()) // close + flush logs
		require.NotContains(t, logs.String(), "merge (")
	})
}
