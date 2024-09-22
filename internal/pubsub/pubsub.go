package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	SimpleQueueDurable   = 0
	SimpleQueueTransient = 1
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonVal, err := json.Marshal(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonVal,
	})
	if err != nil {
		return err
	}
	return nil
}

// declares a queue and binds it to an exchange
func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	var durable, autoDelete, exclusive bool
	if simpleQueueType == SimpleQueueDurable {
		durable = true
		autoDelete = false
		exclusive = false
	} else if simpleQueueType == SimpleQueueTransient {
		durable = false
		autoDelete = true
		exclusive = true
	}

	var queue amqp.Queue
	queue, err = ch.QueueDeclare(
		queueName,
		durable,
		autoDelete,
		exclusive,
		false,
		amqp.Table{
			"x-dead-letter-exchange": "peril_dlx",
		})

	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(
		queue.Name,
		key,
		exchange,
		false,
		nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	fmt.Printf("Queue %s bound to exchange %s with key %s\n", queue.Name, exchange, key)

	return ch, queue, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	err = ch.Qos(10, 0, false)
	if err != nil {
		return err
	}

	deliveryCh, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for msg := range deliveryCh {
			var data T
			err := json.Unmarshal(msg.Body, &data)
			if err != nil {
				fmt.Println("Error unmarshalling message:", err)
				continue
			}
			ack := handler(data)
			if ack == Ack {
				msg.Ack(false)
			} else if ack == NackRequeue {
				msg.Nack(false, true)
			} else if ack == NackDiscard {
				msg.Nack(false, false)
			} else {
				fmt.Println("Invalid ack:", ack)
			}
		}
	}()
	return nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        buf.Bytes(),
	})
	if err != nil {
		return err
	}
	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	deliveryCh, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for msg := range deliveryCh {
			dec := gob.NewDecoder(bytes.NewReader(msg.Body))
			var data T
			err := dec.Decode(&data)
			if err != nil {
				fmt.Println("Error decoding gob:", err)
				continue
			}
			ack := handler(data)
			if ack == Ack {
				msg.Ack(false)
			} else if ack == NackRequeue {
				msg.Nack(false, true)
			} else if ack == NackDiscard {
				msg.Nack(false, false)
			} else {
				fmt.Println("Invalid ack:", ack)
			}
		}
	}()
	return nil
}
