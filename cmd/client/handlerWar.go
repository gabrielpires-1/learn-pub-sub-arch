package main

import (
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	defer fmt.Print("> ")

	return func(recognition gamelogic.RecognitionOfWar) pubsub.AckType {
		outcome, _, _ := gs.HandleWar(recognition)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			log := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    gs.Player.Username,
				Message:     "was not involved in the war",
			}
			pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.Player.Username, log)
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			log := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    gs.Player.Username,
				Message:     "had no units to fight",
			}
			pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.Player.Username, log)
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			log := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    gs.Player.Username,
				Message:     fmt.Sprintf("%v won a war against %v", recognition.Defender.Username, recognition.Attacker.Username),
			}
			pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.Player.Username, log)
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			log := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    gs.Player.Username,
				Message:     fmt.Sprintf("%v won a war against %v", recognition.Attacker.Username, recognition.Defender.Username),
			}
			pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.Player.Username, log)
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			log := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    gs.Player.Username,
				Message:     fmt.Sprintf("A war between %v and %v resulted in a draw", recognition.Attacker.Username, recognition.Defender.Username),
			}
			pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.Player.Username, log)
			return pubsub.Ack
		default:
			fmt.Println("error: unknown war outcome")
			return pubsub.NackDiscard
		}
	}
}
