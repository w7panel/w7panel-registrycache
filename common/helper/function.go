package helper

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
)

func CtxDone(c context.Context) bool {
	if ginC, ok := c.(*gin.Context); ok {
		select {
		case <-ginC.Request.Context().Done():
			return true
		default:
			return false
		}
	}

	return false
}

func CreateDirIfNotExist(dirName string, perm os.FileMode) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err := os.MkdirAll(dirName, perm)
		if err != nil {
			panic(err)
		}
	}
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func GetDifference[T comparable](a, b []T) (onlyA, onlyB []T) {
	// 创建 map 存储 b 的元素
	bMap := make(map[T]bool)
	for _, item := range b {
		bMap[item] = true
	}

	// 创建 map 存储 a 的元素
	aMap := make(map[T]bool)
	for _, item := range a {
		aMap[item] = true
	}

	// 找出只存在于 a 的元素
	for _, item := range a {
		if !bMap[item] {
			onlyA = append(onlyA, item)
		}
	}

	// 找出只存在于 b 的元素
	for _, item := range b {
		if !aMap[item] {
			onlyB = append(onlyB, item)
		}
	}

	return onlyA, onlyB
}
