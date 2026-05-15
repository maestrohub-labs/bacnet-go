// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bacnet

import "fmt"

// Date represents a BACnet Date (application tag 10), encoded as four
// 8-bit fields per ASHRAE 135 § 20.2.12.
//
// Per the spec, any field may carry the wildcard byte 0xFF, meaning
// "unspecified" — common in schedule and calendar objects. The library
// surfaces wildcards verbatim; callers should test for 0xFF on each
// field before using it as a calendar value.
type Date struct {
	// Year is the four-digit calendar year. The wire format is
	// year-1900; the library converts in both directions, so 0xFF on
	// the wire surfaces here as the constant DateWildcardYear (0xFFFF).
	Year uint16
	// Month is 1-12, with 13 meaning "odd months" and 14 meaning "even
	// months" per the spec. 0xFF is the wildcard.
	Month uint8
	// Day is 1-31, with 32 meaning "last day of month". 0xFF is the
	// wildcard.
	Day uint8
	// DayOfWeek is 1 (Monday) through 7 (Sunday). 0xFF is the wildcard.
	DayOfWeek uint8
}

// Wildcard sentinels for Date fields.
const (
	DateWildcardYear  uint16 = 0xFFFF // wire 0xFF
	DateWildcardField uint8  = 0xFF
)

// Time represents a BACnet Time (application tag 11), encoded as four
// 8-bit fields per ASHRAE 135 § 20.2.13. Any field may carry 0xFF as
// a wildcard.
type Time struct {
	Hour       uint8 // 0-23, or 0xFF wildcard
	Minute     uint8 // 0-59, or 0xFF wildcard
	Second     uint8 // 0-59, or 0xFF wildcard
	Hundredths uint8 // 0-99, or 0xFF wildcard
}

// TimeWildcardField is the wildcard sentinel for any Time field.
const TimeWildcardField uint8 = 0xFF

// DecodeDate parses a 4-byte BACnet date payload. The caller is
// responsible for asserting the length before calling.
func DecodeDate(payload []byte) Date {
	d := Date{
		Month:     payload[1],
		Day:       payload[2],
		DayOfWeek: payload[3],
	}
	if payload[0] == 0xFF {
		d.Year = DateWildcardYear
	} else {
		d.Year = uint16(payload[0]) + 1900
	}
	return d
}

// EncodeDate encodes a Date as the 4-byte BACnet payload (without the
// application tag header).
func EncodeDate(d Date) []byte {
	var yearByte uint8
	if d.Year == DateWildcardYear {
		yearByte = 0xFF
	} else {
		// Wire format is year-1900; clamp to 8 bits via the natural cast.
		// Spec says valid range is 1900-2154 (255+1900).
		yearByte = uint8(d.Year - 1900)
	}
	return []byte{yearByte, d.Month, d.Day, d.DayOfWeek}
}

// EncodeDateTag encodes a Date with an application tag (tag 10).
func EncodeDateTag(d Date) []byte {
	data := EncodeDate(d)
	tag := EncodeTag(uint8(TagDate), TagClassApplication, len(data))
	return append(tag, data...)
}

// String renders the date as YYYY-MM-DD (or wildcards as `?`).
func (d Date) String() string {
	yr := "?"
	if d.Year != DateWildcardYear {
		yr = fmt.Sprintf("%04d", d.Year)
	}
	mo := "?"
	if d.Month != DateWildcardField {
		mo = fmt.Sprintf("%02d", d.Month)
	}
	dy := "?"
	if d.Day != DateWildcardField {
		dy = fmt.Sprintf("%02d", d.Day)
	}
	return yr + "-" + mo + "-" + dy
}

// DecodeTime parses a 4-byte BACnet time payload. The caller is
// responsible for asserting the length before calling.
func DecodeTime(payload []byte) Time {
	return Time{
		Hour:       payload[0],
		Minute:     payload[1],
		Second:     payload[2],
		Hundredths: payload[3],
	}
}

// EncodeTime encodes a Time as the 4-byte BACnet payload (without the
// application tag header).
func EncodeTime(t Time) []byte {
	return []byte{t.Hour, t.Minute, t.Second, t.Hundredths}
}

// EncodeTimeTag encodes a Time with an application tag (tag 11).
func EncodeTimeTag(t Time) []byte {
	data := EncodeTime(t)
	tag := EncodeTag(uint8(TagTime), TagClassApplication, len(data))
	return append(tag, data...)
}

// String renders the time as HH:MM:SS.HH (or wildcards as `?`).
func (t Time) String() string {
	field := func(v uint8, width int) string {
		if v == TimeWildcardField {
			return "?"
		}
		return fmt.Sprintf("%0*d", width, v)
	}
	return field(t.Hour, 2) + ":" + field(t.Minute, 2) + ":" + field(t.Second, 2) + "." + field(t.Hundredths, 2)
}
