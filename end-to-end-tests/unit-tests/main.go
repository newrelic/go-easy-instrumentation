package main

import (
	"fmt"
	"unit-tests/pkg1"
	"unit-tests/pkg2"
	"unit-tests/pkg3"
)

func main() {
	fmt.Println("Running complex test app")
	pkg1.Func1()
	a := pkg1.Add(2, 3)
	a = pkg1.Subtract(a, 2)
	pkg2.Func2()
	a = pkg2.Multiply(a, 6)
	a, err := pkg2.Divide(a, 2)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("result: %d\n", a)
	pkg3.Func3()
	str := pkg3.Concat("hello", "world")
	split := pkg3.Split(str, "")
	fmt.Println(split)
}
