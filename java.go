package colfer

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// GenerateJava writes the code into the respective ".java" files.
func GenerateJava(basedir string, structs []*Struct) error {
	t := template.New("java-code").Delims("<:", ":>")
	template.Must(t.Parse(javaCode))
	template.Must(t.New("marshal").Parse(javaMarshal))
	template.Must(t.New("marshal-field").Parse(javaMarshalField))
	template.Must(t.New("unmarshal").Parse(javaUnmarshal))
	template.Must(t.New("unmarshal-field").Parse(javaUnmarshalField))

	for _, s := range structs {
		for _, f := range s.Fields {
			switch f.Type {
			default:
				f.TypeNative = f.Type
			case "bool":
				f.TypeNative = "boolean"
			case "uint32", "int32":
				f.TypeNative = "int"
			case "uint64", "int64":
				f.TypeNative = "long"
			case "float32":
				f.TypeNative = "float"
			case "float64":
				f.TypeNative = "double"
			case "timestamp":
				f.TypeNative = "java.time.Instant"
			case "text":
				f.TypeNative = "String"
			case "binary":
				f.TypeNative = "byte[]"
			}
		}

		pkgdir, err := MakePkgDir(&s.Pkg, basedir)
		if err != nil {
			return err
		}
		s.Pkg.Name = strings.Replace(s.Pkg.Name, "/", ".", -1)

		f, err := os.Create(filepath.Join(pkgdir, s.NameTitle+".java"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := t.Execute(f, s); err != nil {
			return err
		}
	}
	return nil
}

const javaCode = `package <:.Pkg.Name:>;

// This file was generated by colf(1); DO NOT EDIT

/**
 * @author Commander Colfer
 * @see <a href="https://github.com/pascaldekloe/colfer">Colfer's home</a>
 */
public class <:.NameTitle:> implements java.io.Serializable {

	private static final java.nio.charset.Charset utf8 = java.nio.charset.Charset.forName("UTF-8");

<:range .Fields:>	public <:.TypeNative:> <:.Name:>;
<:end:>

<:template "marshal" .:>
<:template "unmarshal" .:>
<:range .Fields:>	public <:.TypeNative:> get<:.NameTitle:>() {
		return this.<:.Name:>;
	}

	public void set<:.NameTitle:>(<:.TypeNative:> value) {
		this.<:.Name:> = value;
	}

<:end:>	/**
	 * Serializes an integer.
	 * @param buf the data destination.
	 * @param x the value.
	 */
	private static void putVarint(java.nio.ByteBuffer buf, int x) {
		while ((x & 0xffffff80) != 0) {
			buf.put((byte) (x | 0x80));
			x >>>= 7;
		}
		buf.put((byte) x);
	}

	/**
	 * Serializes an integer.
	 * @param buf the data destination.
	 * @param x the value.
	 */
	private static void putVarint(java.nio.ByteBuffer buf, long x) {
		while ((x & 0xffffffffffffff80L) != 0) {
			buf.put((byte) (x | 0x80));
			x >>>= 7;
		}
		buf.put((byte) x);
	}

	/**
	 * Deserializes a 32-bit integer.
	 * @param buf the data source.
	 * @return the value.
	 */
	private static int getVarint32(java.nio.ByteBuffer buf) {
		int x = 0;
		int shift = 0;
		while (true) {
			int b = buf.get() & 0xff;
			x |= (b & 0x7f) << shift;
			if (b < 0x80) return x;
			shift += 7;
		}
	}

	/**
	 * Deserializes a 64-bit integer.
	 * @param buf the data source.
	 * @return the value.
	 */
	private static long getVarint64(java.nio.ByteBuffer buf) {
		long x = 0;
		int shift = 0;
		while (true) {
			long b = buf.get() & 0xffL;
			x |= (b & 0x7f) << shift;
			if (b < 0x80) return x;
			shift += 7;
		}
	}

}
`

const javaMarshal = `	/**
	 * Writes in Colfer format.
	 * @param buf the data destination.
	 * @throws java.nio.BufferOverflowException when {@code buf} is too small.
	 */
	public final void marshal(java.nio.ByteBuffer buf) {
		buf.order(java.nio.ByteOrder.BIG_ENDIAN);
		buf.put((byte) 0x80);
<:range .Fields:><:template "marshal-field" .:><:end:>
	}
`

const javaMarshalField = `<:if eq .Type "bool":>
		if (this.<:.Name:>) {
			buf.put((byte) <:.Index:>);
		}
<:else if eq .Type "uint32" "uint64":>
		if (this.<:.Name:> != 0) {
			buf.put((byte) <:.Index:>);
			putVarint(buf, this.<:.Name:>);
		}
<:else if eq .Type "int32":>
		if (this.<:.Name:> != 0) {
			byte header = <:.Index:>;
			int x = this.<:.Name:>;
			if (x < 0) {
				x = -x;
				header |= 0x80;
			}
			buf.put(header);
			putVarint(buf, x);
		}
<:else if eq .Type "int64":>
		if (this.<:.Name:> != 0) {
			byte header = <:.Index:>;
			long x = this.<:.Name:>;
			if (x < 0) {
				x = -x;
				header |= 0x80;
			}
			buf.put(header);
			putVarint(buf, x);
		}
<:else if eq .Type "float32":>
		if (this.<:.Name:> != 0.0f) {
			buf.put((byte) <:.Index:>);
			buf.putFloat(this.<:.Name:>);
		}
<:else if eq .Type "float64":>
		if (this.<:.Name:> != 0.0) {
			buf.put((byte) <:.Index:>);
			buf.putDouble(this.<:.Name:>);
		}
<:else if eq .Type "timestamp":>
		if (this.<:.Name:> != null) {
			long s = this.<:.Name:>.getEpochSecond();
			int ns = this.<:.Name:>.getNano();
			if (! (s == 0 && ns == 0)) {
				byte header = <:.Index:>;
				if (ns != 0) header |= 0x80;
				buf.put(header);
				buf.putLong(s);
				if (ns != 0) buf.putInt(ns);
			}
		}
<:else if eq .Type "text":>
		if (this.<:.Name:> != null && ! this.<:.Name:>.isEmpty()) {
			java.nio.ByteBuffer bytes = utf8.encode(this.<:.Name:>);
			buf.put((byte) 0x08);
			putVarint(buf, bytes.limit());
			buf.put(bytes);
		}
<:else if eq .Type "binary":>
		if (this.<:.Name:> != null && this.<:.Name:>.length != 0) {
			buf.put((byte) 0x09);
			putVarint(buf, this.<:.Name:>.length);
			buf.put(this.<:.Name:>);
		}
<:end:>`

const javaUnmarshal = `	/**
	 * Reads in Colfer format.
	 * @param buf the data source.
	 * @throws java.nio.BufferUnderflowException when {@code buf} is incomplete.
	 */
	public final void unmarshal(java.nio.ByteBuffer buf) {
		int header = buf.get() & 0xff;
		if (header != 0x80)
			throw new IllegalArgumentException("magic header mismatch");

		if (! buf.hasRemaining()) return;
		header = buf.get() & 0xff;
		int field = header & 0x7f;
<:range .Fields:>
		if (field == <:.Index:>) {<:template "unmarshal-field" .:>
			if (! buf.hasRemaining()) return;
			header = buf.get() & 0xff;
			field = header & 0x7f;
		}
<:end:>
		throw new IllegalArgumentException("pending data");
	}
`

const javaUnmarshalField = `<:if eq .Type "bool":>
			this.<:.Name:> = true;
<:else if eq .Type "uint32":>
			this.<:.Name:> = getVarint32(buf);
<:else if eq .Type "uint64":>
			this.<:.Name:> = getVarint64(buf);
<:else if eq .Type "int32":>
			this.<:.Name:> = getVarint32(buf);
			if ((header & 0x80) != 0)
			this.<:.Name:> = (~this.<:.Name:>) + 1;
<:else if eq .Type "int64":>
			this.<:.Name:> = getVarint64(buf);
			if ((header & 0x80) != 0)
			this.<:.Name:> = (~this.<:.Name:>) + 1;
<:else if eq .Type "float32":>
			this.<:.Name:> = buf.getFloat();
<:else if eq .Type "float64":>
			this.<:.Name:> = buf.getDouble();
<:else if eq .Type "timestamp":>
			long s = buf.getLong();
			if ((header & 0x80) == 0) {
				this.<:.Name:> = java.time.Instant.ofEpochSecond(s);
			} else {
				int ns = buf.getInt();
				this.<:.Name:> = java.time.Instant.ofEpochSecond(s, ns);
			}
<:else if eq .Type "text":>
			int length = getVarint32(buf);
			java.nio.ByteBuffer blob = java.nio.ByteBuffer.allocate(length);
			buf.get(blob.array());
			this.<:.Name:> = utf8.decode(blob).toString();
<:else if eq .Type "binary":>
			int length = getVarint32(buf);
			this.<:.Name:> = new byte[length];
			buf.get(<:.Name:>);
<:end:>`
