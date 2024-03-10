package discord

import (
	"time"

	"github.com/hugolgst/rich-go/client"
)

func Init(app_id string) error {
	return client.Login(app_id)
}


func StartSession() error {
	now := time.Now()
	return client.SetActivity(client.Activity{	
		State:      "Heyy!!!",	
		Details:    "I'm running on rich-go :)",	
		LargeImage: "largeimageid",	
		LargeText:  "This is the large image :D",	
		SmallImage: "smallimageid",	
		SmallText:  "And this is the small image",	
		Party: &client.Party{		
			ID:         "-1",		
			Players:    15,		
			MaxPlayers: 24,	
		},	
		Timestamps: &client.Timestamps{		
			Start: &now,	
		},
	})
}