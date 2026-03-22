package messaging

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

// RabbitMQClient handles RabbitMQ publishing and consuming
type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQClient creates a new RabbitMQ client
func NewRabbitMQClient(url string) (*RabbitMQClient, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	slog.Default().Info("connected to RabbitMQ")

	return &RabbitMQClient{
		conn:    conn,
		channel: ch,
	}, nil
}

// DeclareExchange declares an exchange
func (r *RabbitMQClient) DeclareExchange(name string) error {
	return r.channel.ExchangeDeclare(
		name,        // name
		"direct",    // kind
		true,        // durable
		false,       // autoDelete
		false,       // internal
		false,       // noWait
		nil,         // args
	)
}

// DeclareQueue declares a queue
func (r *RabbitMQClient) DeclareQueue(name string) error {
	_, err := r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	return err
}

// BindQueue binds a queue to an exchange
func (r *RabbitMQClient) BindQueue(queueName, exchangeName, routingKey string) error {
	return r.channel.QueueBind(
		queueName,    // queue
		routingKey,   // key
		exchangeName, // exchange
		false,        // noWait
		nil,          // args
	)
}

// amqpCarrier adapts an amqp.Table to the OTel TextMapCarrier interface,
// enabling trace-context injection and extraction via AMQP message headers.
type amqpCarrier amqp.Table

func (c amqpCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c amqpCarrier) Set(key, val string) { c[key] = val }

func (c amqpCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceContext writes the active span from ctx into the AMQP headers table.
// Call before publishing so consumers can continue the same trace.
func InjectTraceContext(ctx context.Context, headers amqp.Table) {
	otel.GetTextMapPropagator().Inject(ctx, amqpCarrier(headers))
}

// ExtractTraceContext reads a span context from AMQP headers and returns a
// derived context. Call at the start of each message-processing goroutine.
func ExtractTraceContext(ctx context.Context, headers amqp.Table) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, amqpCarrier(headers))
}

// Publish publishes a message to an exchange with trace context injected.
func (r *RabbitMQClient) Publish(ctx context.Context, exchange, routingKey string, message []byte) error {
	headers := amqp.Table{}
	InjectTraceContext(ctx, headers)
	return r.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     headers,
			Body:        message,
		},
	)
}

// Consume returns a channel of messages from a queue
func (r *RabbitMQClient) Consume(ctx context.Context, queueName string) (<-chan amqp.Delivery, error) {
	return r.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // autoAck
		false,     // exclusive
		false,     // noLocal
		false,     // noWait
		nil,       // args
	)
}

// ConsumeNoContext returns a channel of messages from a queue (legacy)
func (r *RabbitMQClient) ConsumeNoContext(queueName string) (<-chan amqp.Delivery, error) {
	return r.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // autoAck
		false,     // exclusive
		false,     // noLocal
		false,     // noWait
		nil,       // args
	)
}

// Ack acknowledges a message
func (r *RabbitMQClient) Ack(delivery amqp.Delivery) error {
	return delivery.Ack(false)
}

// Nack negatively acknowledges a message
func (r *RabbitMQClient) Nack(delivery amqp.Delivery, requeue bool) error {
	return delivery.Nack(false, requeue)
}

// DeclareDeadLetterQueue declares a queue with dead letter routing and TTL
func (r *RabbitMQClient) DeclareDeadLetterQueue(name, exchange, routingKey string, ttlMs int64) error {
	args := amqp.Table{
		"x-message-ttl":             ttlMs, // 1 hour TTL: 3600000
		"x-dead-letter-exchange":    exchange,
		"x-dead-letter-routing-key": routingKey,
	}
	
	_, err := r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,  // arguments
	)
	return err
}

// PublishWithRetry publishes a message with a retry counter and trace context in headers.
func (r *RabbitMQClient) PublishWithRetry(ctx context.Context, exchange, routingKey string, message []byte, retryCount int) error {
	headers := amqp.Table{
		"x-retry-count": int32(retryCount),
	}
	InjectTraceContext(ctx, headers)
	
	return r.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     headers,
			Body:        message,
		},
	)
}

// SendToDLQ sends a message to the dead letter queue
func (r *RabbitMQClient) SendToDLQ(ctx context.Context, dlqName, exchange, routingKey string, message []byte) error {
	return r.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        message,
		},
	)
}

// QueueStatus represents queue status information
type QueueStatus struct {
	Name      string
	Messages  int
	Consumers int
}

// GetQueueStatus returns status information about a queue.
//
// AMQP QueueInspect (passive declare) throws a channel-level 404 if the queue
// does not exist, which permanently closes the channel. To prevent that error
// from poisoning the shared publish channel, this method opens a dedicated
// throwaway channel for the inspect call and closes it immediately after.
func (r *RabbitMQClient) GetQueueStatus(queueName string) (*QueueStatus, error) {
	// Open a temporary channel just for this inspection.
	ch, err := r.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("GetQueueStatus: failed to open channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueInspect(queueName)
	if err != nil {
		return nil, err
	}

	return &QueueStatus{
		Name:      queueName,
		Messages:  q.Messages,
		Consumers: q.Consumers,
	}, nil
}

// Close closes the connection
func (r *RabbitMQClient) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}


