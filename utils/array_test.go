package utils

import (
	"fmt"
	"testing"
)

func TestRemove(t *testing.T) {
	arr := []int{234,234,1,34,23,3,435,234,56,5,4,3,24,234};
	_arr := RemoveElement(&arr,234);
	fmt.Printf("%v",_arr);
}