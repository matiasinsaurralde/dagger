package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/dagger/dagger/core"
	"github.com/stretchr/testify/require"
)

func TestSecretScrubWriter_Write(t *testing.T) {
	fsys := fstest.MapFS{
		"mysecret": &fstest.MapFile{
			Data: []byte("my secret file"),
		},
		"subdir/alsosecret": &fstest.MapFile{
			Data: []byte("a subdir secret file \nwith line feed"),
		},
	}
	env := []string{
		"MY_SECRET_ID=my secret value",
	}

	t.Run("scrub files and env", func(t *testing.T) {
		var buf bytes.Buffer
		currentDirPath := "/"
		w, err := NewSecretScrubWriter(&buf, currentDirPath, fsys, env, core.SecretToScrubInfo{
			Envs:  []string{"MY_SECRET_ID"},
			Files: []string{"/mysecret", "/subdir/alsosecret"},
		})
		require.NoError(t, err)

		_, err = fmt.Fprintf(w, "I love to share my secret value to my close ones. But I keep my secret file to myself. As well as a subdir secret file.")
		require.NoError(t, err)
		want := "I love to share *** to my close ones. But I keep *** to myself. As well as ***."
		require.Equal(t, want, buf.String())
	})
	t.Run("do not scrub empty env", func(t *testing.T) {
		env := append(env, "EMPTY_SECRET_ID=")
		currentDirPath := "/"
		fsys := fstest.MapFS{
			"emptysecret": &fstest.MapFile{
				Data: []byte(""),
			},
		}

		var buf bytes.Buffer
		w, err := NewSecretScrubWriter(&buf, currentDirPath, fsys, env, core.SecretToScrubInfo{
			Envs:  []string{"EMPTY_SECRET_ID"},
			Files: []string{"/emptysecret"},
		})
		require.NoError(t, err)

		_, err = fmt.Fprintf(w, "I love to share my secret value to my close ones. But I keep my secret file to myself.")
		require.NoError(t, err)
		want := "I love to share my secret value to my close ones. But I keep my secret file to myself."
		require.Equal(t, want, buf.String())
	})
}

func TestLoadSecretsToScrubFromEnv(t *testing.T) {
	secretValue := "my secret value"
	env := []string{
		fmt.Sprintf("MY_SECRET_ID=%s", secretValue),
		"PUBLIC_STUFF=so public",
	}

	secretToScrub := core.SecretToScrubInfo{
		Envs: []string{
			"MY_SECRET_ID",
		},
	}

	secrets := loadSecretsToScrubFromEnv(env, secretToScrub.Envs)
	require.NotContains(t, secrets, "PUBLIC_STUFF")
	require.Contains(t, secrets, secretValue)
}

func TestLoadSecretsToScrubFromFiles(t *testing.T) {
	const currentDirPath = "/mnt"
	t.Run("/mnt, fs relative, secret absolute", func(t *testing.T) {
		fsys := fstest.MapFS{
			"mysecret": &fstest.MapFile{
				Data: []byte("my secret file"),
			},
			"subdir/alsosecret": &fstest.MapFile{
				Data: []byte("a subdir secret file"),
			},
		}
		secretFilePathsToScrub := []string{"/mnt/mysecret", "/mnt/subdir/alsosecret"}

		secrets, err := loadSecretsToScrubFromFiles(currentDirPath, fsys, secretFilePathsToScrub)
		require.NoError(t, err)
		require.Contains(t, secrets, "my secret file")
		require.Contains(t, secrets, "a subdir secret file")
	})

	t.Run("/mnt, fs relative, secret relative", func(t *testing.T) {
		fsys := fstest.MapFS{
			"mysecret": &fstest.MapFile{
				Data: []byte("my secret file"),
			},
			"subdir/alsosecret": &fstest.MapFile{
				Data: []byte("a subdir secret file"),
			},
		}
		secretFilePathsToScrub := []string{"mysecret", "subdir/alsosecret"}

		secrets, err := loadSecretsToScrubFromFiles(currentDirPath, fsys, secretFilePathsToScrub)
		require.NoError(t, err)
		require.Contains(t, secrets, "my secret file")
		require.Contains(t, secrets, "a subdir secret file")
	})

	t.Run("/mnt, fs absolute, secret relative", func(t *testing.T) {
		fsys := fstest.MapFS{
			"mnt/mysecret": &fstest.MapFile{
				Data: []byte("my secret file"),
			},
			"mnt/subdir/alsosecret": &fstest.MapFile{
				Data: []byte("a subdir secret file"),
			},
		}
		secretFilePathsToScrub := []string{"mnt/mysecret", "mnt/subdir/alsosecret"}

		secrets, err := loadSecretsToScrubFromFiles(currentDirPath, fsys, secretFilePathsToScrub)
		require.NoError(t, err)
		require.Contains(t, secrets, "my secret file")
		require.Contains(t, secrets, "a subdir secret file")
	})
}

var (
	//nolint:typecheck
	//go:embed testdata/id_ed25519
	sshSecretKey string

	//nolint:typecheck
	//go:embed testdata/id_ed25519.pub
	sshPublicKey string
)

func TestScrubSecretWrite(t *testing.T) {
	secrets := []string{
		"secret1",
		"secret with space ",
		sshSecretKey,
		sshPublicKey,
	}
	secrets = splitSecretsByLine(secrets)

	s := "Not secret\nsecret1\nsecret with space\n" + sshSecretKey + "\n" + sshPublicKey
	got := scrubSecretBytes(secrets, []byte(s))
	require.Equal(t, "Not secret\n***\n***\n***\n***\n***\n***\n***\n***\n***\n\n***\n", string(got))
}

// TODO: test multiline vs single line, test mixed lines with secrets in some of them, etc.
// cover missing scenarios from gocov
// cover full inputs without secrets
// performance should be good with mixed scenario (with/without secret)
func TestScrubSecretMultiWrite(t *testing.T) {
	//secrets := []string{
	//"This is secret",
	//}

	// ???
	fsys := fstest.MapFS{
		"mnt/mysecret": &fstest.MapFile{
			Data: []byte("my secret file"),
		},
		"mnt/subdir/alsosecret": &fstest.MapFile{
			Data: []byte("a subdir secret file"),
		},
	}

	env := []string{
		"MY_SECRET_ID=my secret value",
		"SECRET2=another secret value",
	}

	currentDirPath := "/"

	var buf bytes.Buffer
	// w, err := NewSecretScrubWriter(&buf, currentDirPath, fsys, env, core.SecretToScrubInfo{
	w, err := NewSecretScrubWriter(&buf, currentDirPath, fsys, env, core.SecretToScrubInfo{
		Envs: []string{"MY_SECRET_ID", "SECRET2"},
		// Files: []string{"/mysecret", "/subdir/alsosecret"},
	})
	// require.NoError(t, err)
	fmt.Printf("w: %+v, err: %+v\n", w, err)

	_, err = fmt.Fprintf(w, "test123 my secret value abc123")
	// fmt.Printf("n: %+v, err: %+v\n", n, err)

	// fmt.Fprintf(w, "\nanother line my secret value 111")

	// fmt.Fprintf(w, "\nno scrub: another secret value some other string\n")

	out := buf.String()
	fmt.Printf("out: %+v\n", out)

	// want := "I love to share my secret value to my close ones. But I keep my secret file to myself."
	// require.Equal(t, want, buf.String())

}

func BenchmarkScrubSecretMultiWrite(b *testing.B) {
	secrets := []string{"secret value"}
	input := []byte("t111 secret value t222")
	for i := 0; i < b.N; i++ {
		scrubSecretBytes(secrets, input)
	}

}

func BenchmarkScrubSecretMultiWriteNew(b *testing.B) {
	secrets := []string{"secret value"}
	input := []byte("t111 secret value t222\nline without secret\nline with secret value")
	for i := 0; i < b.N; i++ {
		scrubSecretBytesNew(secrets, input)
	}

}
