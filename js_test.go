package bellman

import (
	"fmt"
	"math"
	"testing"

	"github.com/dop251/goja"
)

func TestJS(t *testing.T) {
	fmt.Println("Hello and welcome!")

	vm := goja.New()
	v, err := vm.RunString("2 + 2")
	if err != nil {
		panic(err)
	}
	if num := v.Export().(int64); num != 4 {
		panic(num)
	}
	fmt.Println(v)
}

func Test2(t *testing.T) {
	vm := goja.New()

	// 1. Define the JS function in the VM
	script := `
	function calculateRisk(user) {
	    if (user.age < 18) {
	        return "High (Minor)";
	    }
	    return user.balance > 1000 ? "Low" : "Medium";
	}`

	_, err := vm.RunString(script)
	if err != nil {
		panic(err)
	}

	// 2. Map a Go object to pass into the JS function
	userData := map[string]interface{}{
		"age":     25,
		"balance": 500,
	}

	// 3. Get the function reference from the VM
	// We cast it to goja.Callable so we can invoke it directly
	fn, ok := goja.AssertFunction(vm.Get("calculateRisk"))
	if !ok {
		panic("Not a function")
	}

	// 4. Call the JS function from Go
	// Params: (this context, arguments...)
	result, err := fn(goja.Undefined(), vm.ToValue(userData))
	if err != nil {
		panic(err)
	}

	fmt.Printf("The Risk Level is: %v\n", result) // Output: Medium
}

func Test3(t *testing.T) {
	vm := goja.New()

	// 1. Define a Go function
	// You can use standard Go types; goja handles the conversion!
	goCalculateHypotenuse := func(a, b float64) float64 {
		return math.Sqrt(a*a + b*b)
	}

	// 2. Register the function in the JS VM
	// We are mapping the Go variable to a JS name "getHypotenuse"
	vm.Set("getHypotenuse", goCalculateHypotenuse)

	// 3. Register a more complex function (e.g., a logger)
	vm.Set("goLog", func(msg string) {
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	// 4. Run JS code that calls these Go functions
	script := `
		goLog("Starting calculation...");
		var result = getHypotenuse(3, 4);
		goLog("Result is: " + result);
		result; // Return the result to Go
	`

	val, err := vm.RunString(script)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Final value returned to Go: %v\n", val.Export())
}
