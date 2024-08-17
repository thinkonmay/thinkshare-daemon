package discord

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hugolgst/rich-go/client"
)

type ClientActivityStruct struct {
	State      string `json:"state"`
	Details    string `json:"details"`
	LargeImage string `json:"largeImage"`
	LargeText  string `json:"largeText"`
	SmallImage string `json:"smallImage"`
	SmallText  string `json:"smallText"`
	Buttons    []client.Button `json:"buttons"`
}

func EncodeActivity(activity ClientActivityStruct) string {
	encoded, err := json.Marshal(activity)
	if err != nil {
		panic(err)
	}

	encodedBase64 := string(base64.StdEncoding.EncodeToString(encoded))

	return encodedBase64
}

func DecodeActivity(encoded string) ClientActivityStruct {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		panic(err)
	}

	var activity ClientActivityStruct
	err = json.Unmarshal(decoded, &activity)
	if err != nil {
		panic(err)
	}

	return activity
}

func StartSession(app_id string, activity string) error {
	{
		err := client.Login(app_id)
		if err != nil {
			panic(err)
		}
	}

	activityStruct := DecodeActivity(activity)

	now := time.Now()
	err := client.SetActivity(client.Activity{	
		State:      activityStruct.State,
		Details:    activityStruct.Details,
		LargeImage: activityStruct.LargeImage,
		LargeText:  activityStruct.LargeText,
		SmallImage: activityStruct.SmallImage,
		SmallText:  activityStruct.SmallText,
		// Party: &client.Party{// },
		Timestamps: &client.Timestamps{
			Start: &now,
		},
		Buttons: []*client.Button{
			&client.Button{
				Label: activityStruct.Buttons[0].Label,
				Url: activityStruct.Buttons[0].Url,
			},
		},
	})

	if err != nil {
		fmt.Print(err)
		panic(err)
	}
	return nil
}