package discord

import (
	"fmt"
	"testing"

	"github.com/hugolgst/rich-go/client"
)

func TestMain(t *testing.T) {
	app_id := "1192673864333934772"

	activity := ClientActivityStruct{
		State:      "grupobright.com",
		Details:    "Playing in Bright Cloud",
		LargeImage: "bcg_branco",
		LargeText:  "test",
		SmallImage: "",
		SmallText:  "",
		Buttons: []client.Button{
			{
				Label: "Discord",
				Url:   "https://discord.gg/grupobright",
			},
		},
	}

	encoded := EncodeActivity(activity)
	fmt.Print(encoded)

	{
		err := StartSession(app_id, encoded)
		if err != nil {
			t.Errorf("StartSession failed")
		}
	}

}
