// Package jsonvalue 精确比较、规范化 JSON，并读取十进制整数。
package jsonvalue

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
)

// Equal 按 JSON 语义比较值，忽略对象键顺序和等价数字的写法；无法编码的值不相等。
func Equal(left, right any) bool {
	a, err := Canonical(left)
	if err != nil {
		return false
	}
	b, err := Canonical(right)
	return err == nil && bytes.Equal(a, b)
}

// Canonical 生成按键排序且数值精确规范化的 JSON，保留数组顺序和字符串值。
func Canonical(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return json.Marshal(normalizeNumbers(decoded))
}

// Int64 接受数学意义上的整数，包括小数或指数写法；非整数和越界值返回 false。
func Int64(number json.Number) (int64, bool) {
	if number == "" {
		return 0, false
	}
	if _, err := json.Marshal(number); err != nil {
		return 0, false
	}
	if value, err := number.Int64(); err == nil {
		return value, true
	}
	n := parseDecimal(number)
	if n.digits == "" {
		return 0, true
	}
	if !n.exponent.IsInt64() {
		return 0, false
	}
	exponent := n.exponent.Int64()
	if exponent < 0 || exponent > int64(19-len(n.digits)) {
		return 0, false
	}
	text := n.digits + strings.Repeat("0", int(exponent))
	if n.negative {
		text = "-" + text
	}
	value, err := strconv.ParseInt(text, 10, 64)
	return value, err == nil
}

func normalizeNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		return canonicalNumber(parseDecimal(v))
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeNumbers(item)
		}
	case []any:
		for index, item := range v {
			v[index] = normalizeNumbers(item)
		}
	}
	return value
}

type decimal struct {
	negative bool
	digits   string
	exponent *big.Int
}

func canonicalNumber(number decimal) json.Number {
	if number.digits == "" {
		return json.Number("0")
	}
	digits := number.digits
	text := ""
	// 与 Go JSON 的常规数值范围一致；大指数始终保持紧凑，避免按指数分配内存。
	exponent := number.exponent.Int64()
	if number.exponent.IsInt64() && exponent >= int64(-len(digits)-5) && exponent <= int64(21-len(digits)) {
		position := len(digits) + int(exponent)
		switch {
		case exponent >= 0:
			text = digits + strings.Repeat("0", int(exponent))
		case position > 0:
			text = digits[:position] + "." + digits[position:]
		default:
			text = "0." + strings.Repeat("0", -position) + digits
		}
	} else {
		power := new(big.Int).Add(number.exponent, big.NewInt(int64(len(digits)-1)))
		text = digits[:1]
		if len(digits) > 1 {
			text += "." + digits[1:]
		}
		text += "e"
		if power.Sign() >= 0 {
			text += "+"
		}
		text += power.String()
	}
	if number.negative {
		text = "-" + text
	}
	return json.Number(text)
}

// 输入已由 encoding/json 校验。保留有效数字与十进制指数，不为 1e1000000000 展开零位。
func parseDecimal(number json.Number) decimal {
	mantissa, exponent, _ := strings.Cut(strings.ToLower(string(number)), "e")
	negative := strings.HasPrefix(mantissa, "-")
	mantissa = strings.TrimPrefix(mantissa, "-")
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return decimal{}
	}
	trimmed := strings.TrimRight(digits, "0")
	power := new(big.Int)
	if exponent != "" {
		power.SetString(exponent, 10)
	}
	power.Add(power, big.NewInt(int64(len(digits)-len(trimmed)-len(fraction))))
	return decimal{negative: negative, digits: trimmed, exponent: power}
}
