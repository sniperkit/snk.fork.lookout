/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package git

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	fixtures "gopkg.in/src-d/go-git-fixtures.v3"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"

	"github.com/sniperkit/snk.fork.lookout"
)

type ServiceSuite struct {
	suite.Suite
	Basic  *fixtures.Fixture
	Storer storer.Storer
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}

func (s *ServiceSuite) SetupSuite() {
	require := s.Require()

	err := fixtures.Init()
	require.NoError(err)

	fixture := fixtures.Basic().One()
	fs := fixture.DotGit()
	sto, err := filesystem.NewStorage(fs)
	require.NoError(err)

	s.Basic = fixture
	s.Storer = sto
}

func (s *ServiceSuite) TearDownSuite() {
	require := s.Require()

	err := fixtures.Clean()
	require.NoError(err)
}

func (s *ServiceSuite) TestTreeChanges() {
	require := s.Require()

	dr := NewService(&StorerCommitLoader{s.Storer})
	resp, err := dr.GetChanges(context.TODO(), &lookout.ChangesRequest{
		Head: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: s.Basic.Head.String(),
		},
	})

	require.NoError(err)
	require.NotNil(resp)
}

func (s *ServiceSuite) TestTreeChangesDeleteFile() {
	require := s.Require()

	fixture := fixtures.ByURL("https://github.com/src-d/go-git.git").One()
	fs := fixture.DotGit()
	sto, err := filesystem.NewStorage(fs)
	require.NoError(err)

	dr := NewService(&StorerCommitLoader{sto})
	resp, err := dr.GetChanges(context.TODO(), &lookout.ChangesRequest{
		Base: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: "2275fa7d0c75d20103f90b0e1616937d5a9fc5e6",
		},
		Head: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: "e1d8866ffa78fa16d2f39b0ba5344a7269ee5371",
		},
		WantContents:    true,
		ExcludeVendored: true,
	})

	require.NoError(err)
	require.NotNil(resp)
}

func (s *ServiceSuite) TestTreeFiles() {
	require := s.Require()

	dr := NewService(&StorerCommitLoader{s.Storer})
	resp, err := dr.GetFiles(context.TODO(), &lookout.FilesRequest{
		Revision: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: s.Basic.Head.String(),
		},
	})

	require.NoError(err)
	require.NotNil(resp)
}

func (s *ServiceSuite) TestDiffTree() {
	require := s.Require()

	dr := NewService(&StorerCommitLoader{s.Storer})
	resp, err := dr.GetChanges(context.TODO(), &lookout.ChangesRequest{
		Base: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: "918c48b83bd081e863dbe1b80f8998f058cd8294",
		},
		Head: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: s.Basic.Head.String(),
		},
	})
	require.NoError(err)
	require.NotNil(resp)
}

func (s *ServiceSuite) TestErrorNoRepository() {
	require := s.Require()

	m := &MockCommitLoader{}
	m.On("LoadCommits", mock.Anything, mock.Anything).Once().Return(
		nil, fmt.Errorf("ERROR"))

	dr := NewService(m)

	resp, err := dr.GetChanges(context.TODO(), &lookout.ChangesRequest{
		Head: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: s.Basic.Head.String(),
		},
	})
	require.Error(err)
	require.Nil(resp)
}

func (s *ServiceSuite) TestErrorBadTop() {
	require := s.Require()

	dr := NewService(&StorerCommitLoader{s.Storer})
	resp, err := dr.GetChanges(context.TODO(), &lookout.ChangesRequest{
		Head: &lookout.ReferencePointer{
			InternalRepositoryURL: "file:///myrepo",
			Hash: "979a482e63de12d39675ff741c5a0cf4f068c109",
		},
	})
	require.Error(err)
	require.Nil(resp)
}
