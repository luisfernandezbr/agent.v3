package exporter

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/stretchr/testify/assert"
)

func TestDedupInclusions1(t *testing.T) {
	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "azure"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "azure"
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
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "azure"
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
	logger := hclog.New(hclog.DefaultOptions)

	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(logger, ins)
	if len(got) != 1 {
		t.Fatal("we should merge integrations with the same name, type and user id")
	}
	assert.Equal(t, []string{"1", "2", "3"}, got[0].Config.Inclusions)

}

func TestDedupInclusionsAndMergeUsers2(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeWork
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(logger, ins)
	if len(got) == 1 {
		t.Fatal("should not merge integrations with different types")
	}

	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "2", "3"}, res)

}

func TestDedupInclusionsAndMergeUsers3(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.RefreshToken = "x1"
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.RefreshToken = "x2"
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(logger, ins)
	if len(got) != 2 {
		t.Fatal("should not merge integrations with different configs")
	}

	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "3"}, res)

}

func TestDedupInclusionsAndMergeUsers4(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	ins := []inconfig.IntegrationAgent{}
	in := inconfig.IntegrationAgent{}
	in.ID = "id1"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.AccessToken = "a1"
	in.Config.RefreshToken = "x"
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)

	in = inconfig.IntegrationAgent{}
	in.ID = "id2"
	in.Name = "github"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.AccessToken = "a2"
	in.Config.RefreshToken = "x"
	in.Config.Inclusions = []string{"2", "3"}
	ins = append(ins, in)

	got := dedupInclusionsAndMergeUsers(logger, ins)
	if len(got) != 1 {
		t.Fatal("should merge integrations if refreshtoken is same and accesskey is different")
	}

	var res []string
	for _, in := range got {
		res = append(res, in.Config.Inclusions...)
	}
	assert.Equal(t, []string{"1", "2", "3"}, res)

}
