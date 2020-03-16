package exporter

import (
	"testing"

	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/stretchr/testify/assert"
)

func TestDedupInclusions1(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "azure"
	in.CreatedByUserID = "user1"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "azure"
	in.CreatedByUserID = "user2"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusions(ins)
	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "3"}, res)
}

func TestDedupInclusions2(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "azure"
	in.CreatedByUserID = "user1"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "azure"
	in.CreatedByUserID = "user2"
	in.Type = inconfig.IntegrationTypeWork
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusions(ins)
	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "2", "3"}, res)
}

func TestDedupInclusionsAndMergeUsers1(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.CreatedByUserID = "user1"
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.CreatedByUserID = "user1"
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(ins)
	if len(got) != 1 {
		t.Fatal("we should merge integrations with the same name, type and user id")
	}
	assert.Equal(t, []string{"1", "2", "3"}, got[0].Config.Inclusions)

}

func TestDedupInclusionsAndMergeUsers2(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.CreatedByUserID = "user1"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.CreatedByUserID = "user1"
	in.Type = inconfig.IntegrationTypeWork
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(ins)
	if len(got) == 1 {
		t.Fatal("should not merge integrations with different types")
	}

	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "2", "3"}, res)

}

func TestDedupInclusionsAndMergeUsersUserNotSet(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.CreatedByUserID = "user1"
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.CreatedByUserID = ""
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id3"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.CreatedByUserID = ""
	in.Config.Inclusions = []string{"4"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(ins)
	if len(got) != 3 {
		t.Fatal("should not merge integrations with users not set")
	}
	assert.Equal(t, []string{"1", "2"}, got[0].Config.Inclusions)
	assert.Equal(t, []string{"3"}, got[1].Config.Inclusions)
	assert.Equal(t, []string{"4"}, got[2].Config.Inclusions)
}
