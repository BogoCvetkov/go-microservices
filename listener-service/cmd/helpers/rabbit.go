package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"listener-service/cmd/config"
	"log"
	"net/http"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitPayload struct {
	Endpoint string `endpoint:"endpoint"`
	Method   string `endpoint:"method"`
	Data     any    `json:"data,omitempty"`
}

type JsonResponse struct {
	Err     bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type RouteRequestBody struct {
	Action  string `json:"action"`
	Payload any    `json:"payload,omitempty"`
}

type MethodCallInfo struct {
	Method   string
	Endpoint string
	Body     interface{}
}

func PrepareRabbitConn(app *config.AppConfig) error {

	ch, err := app.Conn.Channel()

	if err != nil {
		return err
	}

	q, err := declareQueue(ch)

	if err != nil {
		return err
	}

	app.Channel = ch
	app.Queue = q

	return nil

}

func declareQueue(ch *amqp.Channel) (*amqp.Queue, error) {
	err := ch.ExchangeDeclare(
		"micro_exchange", // name
		"direct",         // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)

	if err != nil {
		return nil, err
	}

	q, err := ch.QueueDeclare(
		"emails", // name
		false,    // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)

	if err != nil {
		return nil, err
	}

	err = ch.QueueBind(
		q.Name,           // queue name
		"message",        // routing key
		"micro_exchange", // exchange
		false,
		nil)

	return &q, nil
}

func WriteJSON(w http.ResponseWriter, data *JsonResponse, status ...int) error {
	payload := data

	statusCode := http.StatusAccepted
	if len(status) > 0 {
		statusCode = status[0]
	}

	out, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(out)

	return nil
}

func ParseJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1048576 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(data)

	if err != nil {
		return err
	}

	return nil
}

func DecodeJSON(src io.Reader, v any) error {
	err := json.NewDecoder(src).Decode(v)
	if err != nil {
		return err
	}

	return nil
}

func CallModule(info *MethodCallInfo) {
	body, err := json.Marshal(info.Body)
	if err != nil {
		fmt.Println("Failed to marshal body")
		return
	}

	request, err := http.NewRequest(info.Method, info.Endpoint, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Failed to call the auth module")
		return
	}

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to call -->  %s", info.Endpoint))
		return
	}

	defer response.Body.Close()

	var result JsonResponse

	err = DecodeJSON(response.Body, &result)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Failed to parse service response", http.StatusBadRequest)
		return
	}
	fmt.Println(response.StatusCode, result)

	if response.StatusCode >= 400 {
		fmt.Println(string(body), response.StatusCode)
		return
	}

	fmt.Println((*JsonResponse)(&result))

}

func ErrJson(w http.ResponseWriter, msg string, status ...int) {

	payload := JsonResponse{
		Err:     true,
		Message: msg,
	}
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}

	err := WriteJSON(w, &payload, statusCode)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte("Error while parsing JSON"))
	}
}

func ListenOnQueue(app *config.AppConfig) {

	msgs, err := app.Channel.Consume(
		app.Queue.Name, // queue
		"",             // consumer
		true,           // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)

	if err != nil {
		log.Panic(err)
	}

	for d := range msgs {
		log.Printf("Message received from on channel -> %s /n", app.Queue.Name)
		log.Printf(" [x] %s", d.Body)
		RouteMessage(&d)
	}

	return
}

func RouteMessage(data *amqp.Delivery) error {

	var payload RabbitPayload

	err := json.Unmarshal(data.Body, &payload)

	if err != nil {
		fmt.Println("Failed to route message in listener service", err)
	}

	reqInfo := MethodCallInfo{
		Method:   payload.Method,
		Endpoint: payload.Endpoint,
		Body:     payload.Data,
	}

	CallModule(&reqInfo)

	return nil
}
