package utils

import (
	"math/rand"
	"reflect"
	"time"
)

func RandomStringGenerator(n int) string {
	var characters = []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}

func ZeroLen(v interface{}) bool {
	return v == nil ||
		(reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) ||
		(reflect.ValueOf(v).Kind() == reflect.Ptr && !reflect.ValueOf(v).IsNil() && reflect.ValueOf(v).Elem().Len() == 0)
}
