package main

import (
	"lit-life-bot/client"
	"lit-life-bot/config"
	"lit-life-bot/model"
)

func main() {
	model.Database(config.DBMain)

	client.OPQStart()

}
