package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/teca/going-to-mars/payment"
	"google.golang.org/grpc"
)

const (
	FIRST_CLASS_PRICE  = 2000
	SECOND_CLASS_PRICE = 1000
)

type Order struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	TicketClass string `json:"ticketClass"`
	Quantity    int    `json:"quantity"`
	BookingDate string `json:"bookingDate"`
	CreditCard  string `json:"creditCard"`
}

func (o Order) CalculateTicketPrice(quantity int) float32 {

	if o.TicketClass == "firstClass" {
		return float32(FIRST_CLASS_PRICE * quantity)
	} else {
		return float32(SECOND_CLASS_PRICE * quantity)
	}
}

func (o Order) sendPaymentReqMsg(req payment.PaymentReq) *payment.PaymentResp {
	connection, err := grpc.Dial("localhost:8002", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect to the gRPC Server %v", err)
	}
	defer connection.Close()
	client := payment.NewOperationsClient(connection)

	rsp, err := client.ExecutePayment(context.Background(), &req)
	if err != nil {
		log.Printf("Error while receiving a response: %v", err.Error())
	}

	return rsp
}

func (o Order) orderHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%v  %v  %v", r.Method, r.URL.Path, r.Proto)

	var order Order
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&order); err != nil {
		log.Printf("JSON parsing Error: %+v\n", err)
		http.Error(w, "Invalid JSON payload", http.StatusInternalServerError)
		return
	}

	o = order
	log.Printf("Received JSON payload: %+v\n", order)

	ticketPrice := o.CalculateTicketPrice(order.Quantity)

	request := payment.PaymentReq{
		Amount:     ticketPrice,
		CardNumber: order.CreditCard,
	}

	response := o.sendPaymentReqMsg(request)

	var htmlPath string
	if response != nil && response.Success {
		htmlPath = "positive_response.html"
	} else {
		htmlPath = "declined.html"
	}

	filePath := filepath.Join("html-content", htmlPath)

	if w == nil {
		log.Println("http.ResponseWriter is nil")
		return
	}

	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		log.Printf("File not found: %v\n", htmlPath)
		http.Error(w, "HTML file not found", http.StatusInternalServerError)
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading HTML file: %v\n", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(content)

}

func main() {
	log.Println("Visit localhost:8080/mars to buy tickets to Mars (The red planet) ...")

	var order Order
	fs := http.FileServer(http.Dir("html-content"))
	http.Handle("/mars", http.StripPrefix("/mars", fs))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("html-content/images"))))
	http.HandleFunc("/order", order.orderHandler)

	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Printf("Cannot start the server: %v\n", err)
	}
}
