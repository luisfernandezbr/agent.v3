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
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id1"
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
	in.ID = "id1"
	in.Name = "azure"
	in.Type = inconfig.IntegrationTypeSourcecode
	in.Config.Inclusions = []string{"1", "2"}
	ins = append(ins, in)
	in = inconfig.IntegrationAgent{}
	in.ID = "id1"
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
