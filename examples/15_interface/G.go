package interfacesample

import (
	"fmt"
	"strings"
)

// Animal 是 Python 侧拿到的接口；Dog 和 Cat 都没有声明自己实现了它
type Animal interface {
	Kind() string
	Speak(times int) string
}

// 值接收者：Dog 和 *Dog 都是 Animal
type Dog struct{ Name string }

func (d Dog) Kind() string { return "dog" }
func (d Dog) Speak(times int) string {
	return d.Name + ": " + strings.TrimSpace(strings.Repeat("woof ", times))
}

// 指针接收者：只有 *Cat 是 Animal，Cat 值不是
type Cat struct{ Name string }

func (c *Cat) Kind() string { return "cat" }
func (c *Cat) Speak(times int) string {
	if c == nil {
		return "(no cat)"
	}
	return c.Name + ": " + strings.TrimSpace(strings.Repeat("meow ", times))
}

// 返回接口，调用者看不到里面是 Dog 还是 *Cat
func Adopt(kind string, name string) Animal {
	if kind == "cat" {
		return &Cat{Name: name}
	}
	return Dog{Name: name}
}

func Introduce(animal Animal) string {
	return "this is a " + animal.Kind() + ". " + animal.Speak(1)
}

func Nobody() Animal { return nil }
func Stray() Animal  { return (*Cat)(nil) }

// error 是普通接口值，不变成 Python 异常
func Feed(animal Animal, food string) (string, error) {
	if cat, ok := animal.(*Cat); ok && food != "fish" {
		return "", fmt.Errorf("%s only eats fish", cat.Name)
	}
	return animal.Kind() + " ate " + food, nil
}
