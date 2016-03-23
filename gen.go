package colfer

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Generate writes the code into file "Colfer.go".
func Generate(basedir string, structs []*Struct) error {
	pkgT := template.New("go-header").Delims("<:", ":>")
	template.Must(pkgT.Parse(goPackage))

	t := template.New("go-code").Delims("<:", ":>")
	template.Must(t.Parse(goCode))
	template.Must(t.New("marshal-field").Parse(goMarshalField))
	template.Must(t.New("marshal-fieldDecl").Parse(goMarshalFieldDecl))
	template.Must(t.New("marshal-varint").Parse(goMarshalVarint))
	template.Must(t.New("unmarshal-field").Parse(goUnmarshalField))
	template.Must(t.New("unmarshal-varint32").Parse(goUnmarshalVarint32))
	template.Must(t.New("unmarshal-varint64").Parse(goUnmarshalVarint64))

	pkgFiles := make(map[string]*os.File)

	for _, s := range structs {
		pkgdir, err := MakePkgDir(&s.Pkg, basedir)
		if err != nil {
			return err
		}
		s.Pkg.Name = s.Pkg.Name[strings.LastIndexByte(s.Pkg.Name, '/')+1:]

		f, ok := pkgFiles[pkgdir]
		if !ok {
			f, err = os.Create(filepath.Join(pkgdir, "Colfer.go"))
			if err != nil {
				return err
			}
			defer f.Close()

			pkgFiles[pkgdir] = f
			if err = pkgT.Execute(f, s.Pkg); err != nil {
				return err
			}
		}

		if err := t.Execute(f, s); err != nil {
			return err
		}
	}
	return nil
}

func MakePkgDir(p *Package, basedir string) (path string, err error) {
	pkgdir := strings.Replace(p.Name, "/", string(filepath.Separator), -1)
	path = filepath.Join(basedir, pkgdir)
	err = os.MkdirAll(path, os.ModeDir|os.ModePerm)
	return
}

const goPackage = `package <:.Name:>

import (
	"errors"
	"io"
	"math"
	"time"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = math.E
var _ = time.RFC3339

var (
	ErrColferStruct   = errors.New("colfer: struct header mismatch")
	ErrColferField    = errors.New("colfer: unknown field header")
	ErrColferOverflow = errors.New("colfer: varint overflow")
)

`

const goCode = `type <:.Name:> struct {
<:range .Fields:>	<:.Name:>	<:if eq .Type "timestamp":>time.Time<:else if eq .Type "text":>string<:else if eq .Type "binary":>[]byte<:else:><:.Type:><:end:>
<:end:>}

// MarshalTo encodes o as Colfer into buf and returns the number of bytes written.
// If the buffer is too small, MarshalTo will panic.
func (o *<:.Name:>) MarshalTo(buf []byte) int {
	if o == nil {
		return 0
	}

	buf[0] = 0x80
	i := 1
<:range .Fields:><:template "marshal-field" .:><:end:>
	return i
}

// MarshalSize returns the number of bytes that will hold the Colfer serial for sure.
func (o *<:.Name:>) MarshalSize() int {
	if o == nil {
		return 0
	}

	// BUG(pascaldekloe): MarshalBinary panics on documents larger than 2kB due to the
	// fact that MarshalSize is not implemented yet.
	return 2048
}

// MarshalBinary encodes o as Colfer conform encoding.BinaryMarshaler.
// The error return is always nil.
func (o *<:.Name:>) MarshalBinary() (data []byte, err error) {
	data = make([]byte, o.MarshalSize())
	n := o.MarshalTo(data)
	return data[:n], nil
}

// UnmarshalBinary decodes data as Colfer conform encoding.BinaryUnmarshaler.
// The error return options are io.EOF, ErrColferStruct, ErrColferField and ErrColferOverflow.
func (o *<:.Name:>) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return io.EOF
	}

	if data[0] != 0x80 {
		return ErrColferStruct
	}
	if len(data) == 1 {
		return nil
	}

	header := data[1]
	field := header & 0x7f
	i := 2
<:range .Fields:><:template "unmarshal-field" .:><:end:>
	return ErrColferField
}
`

const goMarshalFieldDecl = `		buf[i] = <:printf "0x%02x" .Index:>
		i++`

const goMarshalField = `<:if eq .Type "bool":>
	if o.<:.Name:> {
<:template "marshal-fieldDecl" .:>
	}
<:else if eq .Type "uint32":>
	if x := o.<:.Name:>; x != 0 {
<:template "marshal-fieldDecl" .:>
<:template "marshal-varint":>
	}
<:else if eq .Type "uint64":>
	if x := o.<:.Name:>; x != 0 {
<:template "marshal-fieldDecl" .:>
<:template "marshal-varint":>
	}
<:else if eq .Type "int32":>
	if v := o.<:.Name:>; v != 0 {
<:template "marshal-fieldDecl" .:>
		x := uint32(v)
		if v < 0 {
			x = ^x + 1
			buf[i-1] |= 0x80
		}
<:template "marshal-varint":>
	}
<:else if eq .Type "int64":>
	if v := o.<:.Name:>; v != 0 {
<:template "marshal-fieldDecl" .:>
		x := uint64(v)
		if v < 0 {
			x = ^x + 1
			buf[i-1] |= 0x80
		}
<:template "marshal-varint":>
	}
<:else if eq .Type "float32":>
	if v := o.<:.Name:>; v != 0.0 {
<:template "marshal-fieldDecl" .:>
		x := math.Float32bits(v)
		buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(x>>24), byte(x>>16), byte(x>>8), byte(x)
		i += 4
	}
<:else if eq .Type "float64":>
	if v := o.<:.Name:>; v != 0.0 {
<:template "marshal-fieldDecl" .:>
		x := math.Float64bits(v)
		buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(x>>56), byte(x>>48), byte(x>>40), byte(x>>32)
		buf[i+4], buf[i+5], buf[i+6], buf[i+7] = byte(x>>24), byte(x>>16), byte(x>>8), byte(x)
		i += 8
	}
<:else if eq .Type "timestamp":>
	if v := o.<:.Name:>; !v.IsZero() {
<:template "marshal-fieldDecl" .:>
		s, ns := v.Unix(), v.Nanosecond()
		buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(s>>56), byte(s>>48), byte(s>>40), byte(s>>32)
		buf[i+4], buf[i+5], buf[i+6], buf[i+7] = byte(s>>24), byte(s>>16), byte(s>>8), byte(s)
		i += 8
		if ns != 0 {
			buf[i-9] |= 0x80
			buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(ns>>24), byte(ns>>16), byte(ns>>8), byte(ns)
			i += 4
		}
	}
<:else if eq .Type "text" "binary":>
	if v := o.<:.Name:>; len(v) != 0 {
<:template "marshal-fieldDecl" .:>
		x := uint(len(v))
<:template "marshal-varint":>
		to := i + len(v)
		copy(buf[i:], v)
		i = to
	}
<:end:>`

const goMarshalVarint = `		for x >= 0x80 {
			buf[i] = byte(x | 0x80)
			x >>= 7
			i++
		}
		buf[i] = byte(x)
		i++`

const goUnmarshalField = `
	if field == <:.Index:> {<:if eq .Type "bool":>
		o.<:.Name:> = true
<:else if eq .Type "uint32":>
<:template "unmarshal-varint32":>
		o.<:.Name:> = x
<:else if eq .Type "uint64":>
<:template "unmarshal-varint64":>
		o.<:.Name:> = x
<:else if eq .Type "int32":>
<:template "unmarshal-varint32":>
		if header&0x80 != 0 {
			x = ^x + 1
		}
		o.<:.Name:> = int32(x)
<:else if eq .Type "int64":>
<:template "unmarshal-varint64":>
		if header&0x80 != 0 {
			x = ^x + 1
		}
		o.<:.Name:> = int64(x)
<:else if eq .Type "float32":>
		to := i + 4
		if to < 0 || to > len(data) {
			return io.EOF
		}
		x := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		o.<:.Name:> = math.Float32frombits(x)
		i = to
<:else if eq .Type "float64":>
		to := i + 8
		if to < 0 || to > len(data) {
			return io.EOF
		}
		x := uint64(data[i])<<56 | uint64(data[i+1])<<48 | uint64(data[i+2])<<40 | uint64(data[i+3])<<32
		x |= uint64(data[i+4])<<24 | uint64(data[i+5])<<16 | uint64(data[i+6])<<8 | uint64(data[i+7])
		o.<:.Name:> = math.Float64frombits(x)
		i = to
<:else if eq .Type "timestamp":>
		to := i + 8
		var nsec int64
		if header&0x80 == 0 {
			if to < 0 || to > len(data) {
				return io.EOF
			}
		} else {
			to += 4
			if to < 0 || to > len(data) {
				return io.EOF
			}
			nsec = int64(uint(data[i+8])<<24 | uint(data[i+9])<<16 | uint(data[i+10])<<8 | uint(data[i+11]))
		}
		sec := uint64(data[i])<<56 | uint64(data[i+1])<<48 | uint64(data[i+2])<<40 | uint64(data[i+3])<<32
		sec |= uint64(data[i+4])<<24 | uint64(data[i+5])<<16 | uint64(data[i+6])<<8 | uint64(data[i+7])
		i = to

		o.<:.Name:> = time.Unix(int64(sec), nsec)
<:else if eq .Type "text":>
<:template "unmarshal-varint32":>
		to := i + int(x)
		if to < 0 || to > len(data) {
			return io.EOF
		}
		o.<:.Name:> = string(data[i:to])
		i = to
<:else if eq .Type "binary":>
<:template "unmarshal-varint32":>
		length := int(x)
		to := i + length
		if to < 0 || to > len(data) {
			return io.EOF
		}
		v := make([]byte, length)
		copy(v, data[i:to])
		o.<:.Name:> = v
		i = to
<:end:>
		if i == len(data) {
			return nil
		}
		header = data[i]
		field = header & 0x7f
		i++
	}
`

const goUnmarshalVarint32 = `		var x uint32
		for shift := uint(0); ; shift += 7 {
			if shift >= 32 {
				return ErrColferOverflow
			}
			b := data[i]
			i++
			x |= (uint32(b) & 0x7f) << shift
			if b < 0x80 {
				break
			}
			if i == len(data) {
				return io.EOF
			}
		}`

const goUnmarshalVarint64 = `		var x uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrColferOverflow
			}
			b := data[i]
			i++
			x |= (uint64(b) & 0x7f) << shift
			if b < 0x80 {
				break
			}
			if i == len(data) {
				return io.EOF
			}
		}`
