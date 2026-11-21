package closure

import "testing"

// tcmpEnum_ is an enum type of the program's shape without its own compareTo (post-flip enumcmp
// deletes the forwarders): the natural order compares it by java.lang.Enum's compareTo.
type tcmpEnum_ uint8

var tcmpEnum_enum_ = [3]Enum{{name_: "A", ordinal_: 0}, {name_: "B", ordinal_: 1}, {name_: "C", ordinal_: 2}}

func (this tcmpEnum_) enum_() *Enum {
	if this == 0 {
		return nil
	}
	return &tcmpEnum_enum_[this-1]
}

func (this tcmpEnum_) ordinal() int32 { return this.enum_().ordinal_ }

func TestCompareNaturalEnum(t *testing.T) {
	for a := tcmpEnum_(1); a <= 3; a++ {
		for b := tcmpEnum_(1); b <= 3; b++ {
			want := a.enum_().compareTo(b)
			if got := compareNatural_(a, b); got != want || (got < 0) != (a < b) || (got == 0) != (a == b) {
				t.Errorf("compareNatural_(%d, %d) = %d, want %d", a, b, got, want)
			}
		}
	}
}
