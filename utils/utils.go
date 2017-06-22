// TODO: Extract as separate package?
package utils

import (
	"errors"
	"os"

	"github.com/streadway/amqp"
)

func FromEnvWithDefault(envName, defaultValue string) string {
	data := os.Getenv(envName)
	if data == "" {
		data = defaultValue
	}
	return data
}

func CheckRequiredEnvVars(requiredEnvVars []string) error {
	for i := range requiredEnvVars {
		env_name := requiredEnvVars[i]
		value := os.Getenv(env_name)
		if value == "" {
			return errors.New("You must pass the environment variable: " + env_name)
		}
	}
	return nil
}

func DeclareExchange(ch *amqp.Channel, name string) error {
	return ch.ExchangeDeclare(
		name,     // name
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
}

func DeclareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when usused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func BindQueue(ch *amqp.Channel, queue, exchange string) error {
	return ch.QueueBind(
		queue,    // queue name
		queue,    // routing key
		exchange, // exchange
		false,
		nil,
	)
}

func ConsumeMessages(ch *amqp.Channel, queue string) (<-chan amqp.Delivery, error) {
	return ch.Consume(
		queue, // queue
		"",    // consumer
		false, // auto ack
		false, // exclusive
		false, // no local
		false, // no wait
		nil,   // args
	)
}
