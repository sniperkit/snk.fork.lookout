/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/sourcegraph/go-vcsurl.v1"
	"gopkg.in/src-d/go-billy.v4/memfs"

	"github.com/sniperkit/snk.fork.lookout"
)

func TestLibrary_Sync(t *testing.T) {
	require := require.New(t)
	library := NewLibrary(memfs.New())
	syncer := NewSyncer(library)

	url, _ := vcsurl.Parse("http://github.com/sniperkit/snk.fork.lookout")
	err := syncer.Sync(context.TODO(), lookout.ReferencePointer{
		InternalRepositoryURL: url.CloneURL,
		ReferenceName:         "refs/pull/1/head",
		Hash:                  "80a9810a027672a098b07efda3dc305409c9329d",
	})

	require.NoError(err)
	has, err := library.Has(url)
	require.NoError(err)
	require.True(has)
}
