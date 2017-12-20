package checking

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// CheckTask is the struct of once node check.
type CheckTask struct {
	sync.Mutex
	URL    string
	Conn   *amqp.Connection
	Ch     *amqp.Channel
	Reason error
}

func (t *CheckTask) getConn() error {
	var err error
	var retryTimeExceed bool

Retry:
	for i := 1; i <= 3; i++ {

		// build connection timeout 5 second
		// max timeout  5 * 3 + 3 * 1 = 18s
		t.Conn, err = amqp.DialConfig(t.URL, amqp.Config{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, 5*time.Second)
			},
		})

		if err != nil {
			time.Sleep(time.Second * 1)

			if i == 3 {
				retryTimeExceed = true
			}
			continue
		}

		break Retry
	}

	if retryTimeExceed {
		return fmt.Errorf("get [%s] connection error: %s", t.URL, err)
	}

	return nil
}

func (t *CheckTask) getChan() error {
	var err error

	err = t.getConn()
	if err != nil {
		return err
	}

	t.Ch, err = t.Conn.Channel()
	if err != nil {
		return fmt.Errorf("get [%s] channel error: %s", t.URL, err)
	}

	return nil
}

func (t *CheckTask) discardConn() {
	if t.Ch != nil {
		t.Ch.Close()
		t.Ch = nil
	}
	if t.Conn != nil {
		t.Conn.Close()
		t.Conn = nil
	}
}

func (t *CheckTask) messageCheck(ctx context.Context) error {
	var err error
	wg := sync.WaitGroup{}

	qName := "MaxQ.test.q" + strconv.Itoa(rand.Intn(100000))
	tQ, err := t.Ch.QueueDeclare(
		qName,
		false,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to declare queue: %s", err)
	}

	exName := "MaxQ.test.ex" + strconv.Itoa(rand.Intn(100000))
	err = t.Ch.ExchangeDeclare(
		exName,
		"fanout",
		false,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to declare exchange: %s", err)
	}

	err = t.Ch.QueueBind(
		tQ.Name,
		"",
		exName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to bind queue: %s", err)
	}

	msgs, err := t.Ch.Consume(
		tQ.Name,
		"",
		true,
		true,
		false,
		false,
		nil,
	)

	go func() {
		wg.Add(1)

		select {
		case msg := <-msgs:
			if string(msg.Body) == "checkMessage" {
				break
			}
		case <-ctx.Done():
			break
		}

		wg.Done()
	}()

	err = t.Ch.Publish(
		exName,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("checkMessage"),
		})
	if err != nil {
		return fmt.Errorf("Publish message failed: %s", err)
	}

	wg.Wait()

	return nil
}

// check begin
func (t *CheckTask) Check() error {
	defer t.discardConn()

	err := t.getChan()
	if err != nil {
		t.Reason = err
		return err
	}
	// max timeout 10s
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(10))
	defer cancel()

	err = t.messageCheck(ctx)
	if err != nil {
		err = fmt.Errorf("message check failed: %s", err)
		t.Reason = err

		return err
	}

	if ctx.Err() != nil {
		err = fmt.Errorf("consume message timeout")
		t.Reason = err
		return err
	}

	return nil
}

// NewCheckTask return new node check task.
func NewCheckTask(url string) *CheckTask {
	return &CheckTask{
		sync.Mutex{},
		url,
		nil,
		nil,
		nil,
	}
}
