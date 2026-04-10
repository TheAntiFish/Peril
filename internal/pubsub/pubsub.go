package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int
type Acktype int

const (
    Durable SimpleQueueType = iota // 0
    Transient                      // 1
)

const (
	Ack Acktype = iota // 0
	NackRequeue                // 1
	NackDiscard                 // 2
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func SubscribeJSON[T any](
    conn *amqp.Connection,
    exchange,
    queueName,
    key string,
    queueType SimpleQueueType, // an enum to represent "durable" or "transient"
    handler  func(T) Acktype,
) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := json.Unmarshal(data, &val)
		return val, err
	})
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	table := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}

	queue, err := ch.QueueDeclare(
		queueName,
		queueType == Durable,
		queueType == Transient,
		queueType == Transient,
		false,
		table,
	)
	if err != nil {
		ch.Close()
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(
		queueName,
		key,
		exchange,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}

func PublishGob(ch *amqp.Channel, exchange, key string, val any) error {
	var body bytes.Buffer
	enc := gob.NewEncoder(&body)
	err := enc.Encode(val)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        body.Bytes(),
	})
}

func SubscribeGob[T any](
    conn *amqp.Connection,
    exchange,
    queueName,
    key string,
    queueType SimpleQueueType, // an enum to represent "durable" or "transient"
    handler  func(T) Acktype,
) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var val T
		err := gob.NewDecoder(bytes.NewReader(data)).Decode(&val)
		return val, err
	})
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) Acktype,
	unmarshaller func([]byte) (T, error),
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	err = ch.Qos(10, 0, false)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer ch.Close()
		for msg := range msgs {
			val, err := unmarshaller(msg.Body)
			if err != nil {
				// handle error, maybe log it and nack the message
				msg.Nack(false, false)
				continue
			}

			switch handler(val) {
			case Ack:
				msg.Ack(false)
				fmt.Println("Ack Called")
			case NackRequeue:
				msg.Nack(false, true)
				fmt.Println("NackRequeue Called")
			case NackDiscard:
				msg.Nack(false, false)
				fmt.Println("NackDiscard Called")
			}
		}
	}()

	return nil
}
