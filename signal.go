package flo

import "errors"

type Signal[T any] []func(T)error

func (s *Signal[T]) On(f func(T) error) {
	*s = append(*s, f)
}

func (s *Signal[T]) emit(val T) error {
	var result error

	for _, f := range *s {
		err := f(val)
		if err != nil {
			if result == nil {
				result = err
			} else {
				result = errors.Join(result, err)
			}
		}
	}

	return result
}
