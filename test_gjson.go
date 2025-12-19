package main
import (
"fmt"
"github.com/tidwall/gjson"
)
func main() {
json := `{"contents":[{"role":"user"},{"role":"model"}]}`
fmt.Println("Attempt 1:", gjson.Get(json, "contents.@last.role").String())
fmt.Println("Attempt 2:", gjson.Get(json, "contents.-1.role").String())
fmt.Println("Attempt 3:", gjson.Get(json, "contents.#.role").Array())
    // Get last element from array
    res := gjson.Get(json, "contents")
    if res.IsArray() {
        arr := res.Array()
        if len(arr) > 0 {
            fmt.Println("Last explicit:", arr[len(arr)-1].Get("role").String())
        }
    }
}
