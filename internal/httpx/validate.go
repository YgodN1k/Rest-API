package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxBodyBytes — максимальный размер тела JSON-запроса.
const MaxBodyBytes = 1 << 20 // 1 MB

// DecodeJSON читает тело запроса в dst. Строгий режим:
//   - тело не больше MaxBodyBytes;
//   - неизвестные поля запрещены;
//   - ровно один JSON-объект (без мусора после него).
//
// Возвращаемая ошибка безопасна для показа клиенту (ответ 400).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

// decodeError переводит ошибки encoding/json в понятные клиенту сообщения.
func decodeError(err error) error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		maxErr    *http.MaxBytesError
	)
	switch {
	case errors.Is(err, io.EOF):
		return errors.New("request body must not be empty")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("request body contains malformed JSON")
	case errors.As(err, &typeErr):
		if typeErr.Field != "" {
			return fmt.Errorf("field %q has invalid type", typeErr.Field)
		}
		return errors.New("request body has invalid type")
	case errors.As(err, &maxErr):
		return fmt.Errorf("request body must not be larger than %d bytes", maxErr.Limit)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json не экспортирует тип для этой ошибки.
		return fmt.Errorf("unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
	default:
		return errors.New("invalid request body")
	}
}
