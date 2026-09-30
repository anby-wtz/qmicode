package safe_test

import (
	"qmicode-go/app/config"
	"qmicode-go/app/safe"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	s := safe.SafeConfig{
		Environment: safe.SafeEnvironmentRealSystem,
		NeedConfirm: false,
	}
	s.SaveToDefault(config.DefaultConfig)
	u := safe.SafeConfig{}
	u.LoadFromDefault(config.DefaultConfig)
	if u.Environment != s.Environment {
		t.Error("Load error")
	}
	if u.NeedConfirm != s.NeedConfirm {
		t.Error("Load error")
	}

}
