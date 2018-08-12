/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package git

import (
	"context"
	"fmt"

	"gopkg.in/src-d/go-git-fixtures.v3"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"

	"github.com/sniperkit/snk.fork.lookout"
)

func Example() {
	if err := fixtures.Init(); err != nil {
		panic(err)
	}
	defer fixtures.Clean()

	fixture := fixtures.Basic().One()
	fs := fixture.DotGit()
	storer, err := filesystem.NewStorage(fs)
	if err != nil {
		panic(err)
	}

	// Create the git service with a repository loader that allows it to find
	// a repository by ID.
	srv := NewService(&StorerCommitLoader{storer})
	changes, err := srv.GetChanges(context.Background(),
		&lookout.ChangesRequest{
			Base: &lookout.ReferencePointer{
				InternalRepositoryURL: "file:///myrepo",
				Hash: "af2d6a6954d532f8ffb47615169c8fdf9d383a1a",
			},
			Head: &lookout.ReferencePointer{
				InternalRepositoryURL: "file:///myrepo",
				Hash: "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
			},
		})
	if err != nil {
		panic(err)
	}

	for changes.Next() {
		change := changes.Change()
		fmt.Printf("changed: %s\n", change.Head.Path)
	}

	if err := changes.Err(); err != nil {
		panic(err)
	}

	if err := changes.Close(); err != nil {
		panic(err)
	}

	// Output: changed: go/example.go
	// changed: php/crappy.php
	// changed: vendor/foo.go
}
