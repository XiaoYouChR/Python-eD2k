package protocol

import "testing"

func TestHashFromDataMD4Vectors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "empty", data: "", want: "31D6CFE0D16AE931B73C59D7E0C089C0"},
		{name: "a", data: "a", want: "BDE52CB31DE33E46245E05FBDBD6FB24"},
		{name: "abc", data: "abc", want: "A448017AAF21D8525FC10AE87AA6729D"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := HashFromData([]byte(tc.data))
			if err != nil {
				t.Fatalf("HashFromData error: %v", err)
			}
			if got.String() != tc.want {
				t.Fatalf("md4 mismatch: got %s want %s", got.String(), tc.want)
			}
		})
	}
}
