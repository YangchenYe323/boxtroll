package bilibili

import "slices"

type response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TTL     int    `json:"ttl"`
	Data    *T     `json:"data"`
}

func (r *response[T]) DataOrError(acceptedCodes ...int) (*T, error) {
	if len(acceptedCodes) == 0 {
		acceptedCodes = []int{0}
	}

	if slices.Contains(acceptedCodes, r.Code) {
		return r.Data, nil
	}

	return nil, &APIError{
		Code:    r.Code,
		Message: r.Message,
	}
}
