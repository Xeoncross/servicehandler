package servicehandler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
)

type serviceMethod struct {
	params []reflect.Type
	index  int
	method reflect.Value
}

func Wrap(service interface{}) http.Handler {

	// Improve performance (and clarity) by pre-computing needed variables
	serviceType := reflect.TypeOf(service)

	var methods = make(map[string]*serviceMethod)

	for i := 0; i < serviceType.NumMethod(); i++ {
		methodType := serviceType.Method(i)
		method := methodType.Func

		fmt.Printf("%v has %d params\n", methodType.Name, method.Type().NumIn())

		in := make([]reflect.Type, method.Type().NumIn())

		for j := 0; j < method.Type().NumIn(); j++ {
			in[i] = method.Type().In(j)
		}

		name := methodType.Name
		methods[name] = &serviceMethod{params: in, index: i, method: method}
	}

	// Cache setup, now get setup to process values

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.RequestURI())

		fmt.Printf("Calling %v\n", name)

		method, ok := methods[name]

		if !ok {
			http.Error(w, fmt.Sprintf("Unknown method %s", name), http.StatusNotFound)
			return
		}

		in := make([]reflect.Value, len(method.params))

		for i, paramType := range method.params {

			// Create a new instance of each param
			var object reflect.Value

			switch paramType.Kind() {
			case reflect.Struct:
				object = newReflectType(paramType).Elem()
			case reflect.Ptr:
				object = newReflectType(paramType)
			case reflect.String:
				object = reflect.New(paramType).Elem()
			default:
				fmt.Printf("Unknown type: %s", paramType.Kind().String())
			}

			if object.CanInterface() {
				i := object.Interface()
				err := json.NewDecoder(r.Body).Decode(&i)
				if err != nil {
					log.Fatal(err)
				}

				// fmt.Printf("%#v\n", object.Interface())
				// fmt.Printf("%#v\n", i.(*ProviderA))

			} else if object.CanSet() {
				// TODO handle each type of variable
				// var b []byte
				// err := json.NewDecoder(strings.NewReader(`{"a":"foo"}`)).Decode(&b)
				// if err != nil {
				// 	t.Error(err)
				// }
				// object.Set(reflect.ValueOf(b))
			}

			in[i] = object
		}

		response := method.method.Call(in)

		var results []interface{}

		for _, item := range response {
			if err, ok := item.Interface().(error); ok {
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
			} else {
				results = append(results, item.Interface())
			}
		}

		if len(results) > 0 {
			if len(results) == 1 {
				JSON(w, results[0])
			} else {
				JSON(w, results)
			}
		}

	})
}

func newReflectType(t reflect.Type) reflect.Value {
	// Dereference pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return reflect.New(t)
}

// JSON response helper
func JSON(w http.ResponseWriter, i interface{}) {
	var err error
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(i)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		// http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
