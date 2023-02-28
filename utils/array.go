package utils




func RemoveElement(arr *[]int,val int) []int {

	index := make([]int,0);
	for i,v := range *arr {
		if v == val {
			index = append(index, i)
		}
	}

	if len(index) == 0 {
		return *arr;
	}


	last := 0
	arrCopy := make([]int, 0)
	for _,v := range index {
		arrCopy = append(arrCopy, (*arr)[last:v]...)
		last = v + 1
	}
	

	return arrCopy;
}