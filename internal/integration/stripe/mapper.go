package stripe

import (
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/google/uuid"
)

// ToDomainCustomer преобразует клиента Stripe в доменную модель
func ToDomainCustomer(stripeCustomer *CustomerResponse, existingID uuid.UUID) domain.Customer {
	var customerID uuid.UUID
	if existingID != uuid.Nil {
		customerID = existingID
	} else {
		customerID = uuid.New()
	}

	// Время создания в Unix timestamp
	createdAt := time.Unix(stripeCustomer.Created, 0)

	return domain.Customer{
		ID:         customerID,
		Email:      stripeCustomer.Email,
		Name:       stripeCustomer.Name,
		Phone:      stripeCustomer.Phone,
		ExternalID: stripeCustomer.ID, // Stripe ID как внешний ID
		Metadata:   stripeCustomer.Metadata,
		CreatedAt:  createdAt,
		UpdatedAt:  time.Now(),
	}
}

// ToDomainPayment преобразует платеж Stripe в доменную модель
func ToDomainPayment(stripePayment *PaymentIntentResponse, customerID uuid.UUID, existingID uuid.UUID) domain.Payment {
	var paymentID uuid.UUID
	if existingID != uuid.Nil {
		paymentID = existingID
	} else {
		paymentID = uuid.New()
	}

	// Преобразуем сумму из копеек обратно в рубли
	amount := float64(stripePayment.Amount) / 100.0

	// Время создания в Unix timestamp
	createdAt := time.Unix(stripePayment.Created, 0)

	// Статус платежа
	status := StripeStatus(stripePayment.Status)

	// Сообщение об ошибке, если есть
	var errorMessage string
	if stripePayment.LastPaymentError != nil {
		errorMessage = stripePayment.LastPaymentError.Message
	}

	// Создаем или обновляем метаданные
	metadata := make(map[string]string)
	if stripePayment.Metadata != nil {
		for k, v := range stripePayment.Metadata {
			metadata[k] = v
		}
	}
	// Добавляем Stripe ID как метаданные
	metadata["stripe_payment_intent_id"] = stripePayment.ID
	metadata["stripe_client_secret"] = stripePayment.ClientSecret

	return domain.Payment{
		ID:            paymentID,
		CustomerID:    customerID,
		Amount:        amount,
		Currency:      stripePayment.Currency,
		Description:   stripePayment.Description,
		Status:        status,
		MethodID:      uuid.Nil, // ID метода платежа не предоставляется в этом примере
		MethodType:    "card",   // Предполагаем, что это карта
		TransactionID: stripePayment.ID,
		ReceiptURL:    stripePayment.ReceiptURL,
		ErrorMessage:  errorMessage,
		Metadata:      metadata,
		CreatedAt:     createdAt,
		UpdatedAt:     time.Now(),
	}
}

// FromDomainCustomer преобразует доменную модель клиента в параметры для Stripe
func FromDomainCustomer(customer domain.Customer) map[string]string {
	params := make(map[string]string)

	params["email"] = customer.Email

	if customer.Name != "" {
		params["name"] = customer.Name
	}

	if customer.Phone != "" {
		params["phone"] = customer.Phone
	}

	// Добавляем метаданные
	if customer.Metadata != nil {
		for key, value := range customer.Metadata {
			params["metadata["+key+"]"] = value
		}
	}

	// Добавляем ID из нашей системы как метаданные
	params["metadata[payment_service_customer_id]"] = customer.ID.String()

	return params
}

// FromDomainPayment преобразует доменную модель платежа в параметры для Stripe
func FromDomainPayment(payment domain.Payment) map[string]string {
	params := make(map[string]string)

	// Преобразуем сумму из рублей в копейки для Stripe
	amountInSmallestUnit := int64(payment.Amount * 100)

	params["amount"] = string(amountInSmallestUnit)
	params["currency"] = payment.Currency

	if payment.Description != "" {
		params["description"] = payment.Description
	}

	// Добавляем метаданные
	if payment.Metadata != nil {
		for key, value := range payment.Metadata {
			params["metadata["+key+"]"] = value
		}
	}

	// Добавляем ID из нашей системы как метаданные
	params["metadata[payment_service_payment_id]"] = payment.ID.String()
	params["metadata[customer_id]"] = payment.CustomerID.String()

	return params
}
