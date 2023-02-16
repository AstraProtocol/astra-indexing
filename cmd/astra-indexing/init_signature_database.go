package main

//
//import (
//	"fmt"
//	"github.com/akrylysov/pogreb"
//	"io/ioutil"
//	"log"
//)
//
//func main() {
//
//	db, err := pogreb.Open("4bytes.db", nil)
//	if err != nil {
//		panic(err)
//		return
//	}
//	defer db.Close()
//
//	dir := "/Users/lap02341/resource/golang/4bytes/signatures/"
//	files, err := ioutil.ReadDir(dir)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	i := 0
//
//	for _, file := range files {
//		i += 1
//		if i%1000 == 0 {
//			fmt.Println(i)
//		}
//		fileName := dir + file.Name()
//		body, err := ioutil.ReadFile(fileName)
//		if err != nil {
//			log.Fatalf("unable to read file: %v", err)
//		}
//		err = db.Put([]byte(file.Name()), body)
//		if err != nil {
//			fmt.Println(err)
//		}
//	}
//
//}
