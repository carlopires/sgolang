#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""float-oracle.py - the reference semantics of S-Go's float64 (D188-D190, D194).

Every result is computed in exact rational arithmetic (fractions.Fraction) and rounded to binary64 by
this file's own round-to-nearest-even, so the oracle depends neither on the compiler, nor on Go's
code, nor on the host's floating-point unit. A value is written as 16 hex digits of its bits, or
"NaN": a NaN's sign and payload are not observable in S-Go (they differ between amd64 and arm64), so
the oracle has exactly one NaN.

    float-oracle.py selftest [N]   check the oracle against the host's IEEE arithmetic (no fusion)
    float-oracle.py cases SEED N   print N generated cases: "op args -> result"

Operations: add sub mul div neg (D188); fma (math.FMA: one rounding) and muladd (a + x*y in S-Go:
two roundings, never fused); eq lt le (D190: false with NaN, -0 == +0); toint (D189: truncation
toward zero, "Overflow" for NaN, infinities, and values outside int); fromint (D189: rounded);
parse (strconv.ParseFloat of a decimal text: correctly rounded).
"""

import random
import struct
import sys
from fractions import Fraction

NAN = "NaN"
SIGN = 1 << 63
EXPMASK = 0x7FF << 52
FRACMASK = (1 << 52) - 1
POSINF = 0x7FF << 52
NEGINF = SIGN | POSINF
INT_MIN, INT_MAX = -(1 << 63), (1 << 63) - 1


def is_nan(b):
    return b == NAN


def is_inf(b):
    return b != NAN and b & ~SIGN == POSINF


def neg_bit(b):
    return b != NAN and bool(b & SIGN)


def is_zero(b):
    return b != NAN and b & ~SIGN == 0


def to_fraction(b):
    """The exact value of a finite double."""
    e = (b >> 52) & 0x7FF
    f = b & FRACMASK
    if e == 0:
        q = Fraction(f, 1 << 1074)
    else:
        q = Fraction(f | (1 << 52)) * Fraction(2) ** (e - 1075)
    return -q if b & SIGN else q


def rne(q):
    """q rounded to an integer, ties to even."""
    fl = q.numerator // q.denominator
    rem = q - fl
    if rem > Fraction(1, 2) or (rem == Fraction(1, 2) and fl & 1):
        fl += 1
    return fl


def from_fraction(q, negzero=False):
    """The double nearest q (ties to even); a zero result keeps the sign negzero asks for."""
    if q == 0:
        return SIGN if negzero else 0
    s = SIGN if q < 0 else 0
    a = abs(q)
    e = a.numerator.bit_length() - a.denominator.bit_length()
    if Fraction(2) ** e > a:
        e -= 1
    E = max(e, -1022)
    m = rne(a / Fraction(2) ** (E - 52))
    if m == 1 << 53:
        m, E = 1 << 52, E + 1
    if E > 1023:
        return s | POSINF
    if m == 0:
        return s  # underflow to a signed zero
    if m < 1 << 52:
        return s | m  # subnormal
    return s | ((E + 1023) << 52) | (m - (1 << 52))


def add(x, y):
    if is_nan(x) or is_nan(y):
        return NAN
    if is_inf(x) and is_inf(y):
        return x if neg_bit(x) == neg_bit(y) else NAN
    if is_inf(x):
        return x
    if is_inf(y):
        return y
    q = to_fraction(x) + to_fraction(y)
    if q == 0:  # exact zero: -0 only for (-0) + (-0) under round-to-nearest
        return SIGN if (is_zero(x) and is_zero(y) and neg_bit(x) and neg_bit(y)) else 0
    return from_fraction(q)


def neg(x):
    return NAN if is_nan(x) else x ^ SIGN


def sub(x, y):
    return add(x, neg(y))


def mul(x, y):
    if is_nan(x) or is_nan(y):
        return NAN
    s = neg_bit(x) != neg_bit(y)
    if is_inf(x) or is_inf(y):
        if is_zero(x) or is_zero(y):
            return NAN
        return (SIGN if s else 0) | POSINF
    return from_fraction(to_fraction(x) * to_fraction(y), negzero=s)


def div(x, y):
    if is_nan(x) or is_nan(y):
        return NAN
    s = neg_bit(x) != neg_bit(y)
    if is_inf(x):
        return NAN if is_inf(y) else (SIGN if s else 0) | POSINF
    if is_inf(y):
        return SIGN if s else 0
    if is_zero(y):
        return NAN if is_zero(x) else (SIGN if s else 0) | POSINF
    return from_fraction(to_fraction(x) / to_fraction(y), negzero=s)


def fma(x, y, z):
    """math.FMA: x*y + z with one rounding."""
    p = mul(x, y)
    if is_nan(x) or is_nan(y) or is_nan(z) or is_inf(x) or is_inf(y) or is_inf(z) or p == NAN:
        return add(p, z)  # the special cases are those of the two-step form
    q = to_fraction(x) * to_fraction(y) + to_fraction(z)
    if q == 0:
        pz = is_zero(p) and neg_bit(x) != neg_bit(y)  # the sign of the exact product's zero
        return SIGN if (pz and is_zero(z) and neg_bit(z)) else 0
    return from_fraction(q)


def muladd(a, x, y):
    """a + x*y in S-Go source: the product is rounded, then the sum (D188, never fused)."""
    return add(a, mul(x, y))


def eq(x, y):
    if is_nan(x) or is_nan(y):
        return False
    return to_fraction(x) == to_fraction(y) if not (is_inf(x) or is_inf(y)) else x == y


def key(x):
    return Fraction(-1 if neg_bit(x) else 1) * (10 ** 400) if is_inf(x) else to_fraction(x)


def lt(x, y):
    return not (is_nan(x) or is_nan(y)) and key(x) < key(y)


def le(x, y):
    return not (is_nan(x) or is_nan(y)) and key(x) <= key(y)


def toint(x):
    """int(x): truncation toward zero; Overflow for NaN, the infinities, and out of range (D189)."""
    if is_nan(x) or is_inf(x):
        return "Overflow"
    q = to_fraction(x)
    t = q.numerator // q.denominator if q >= 0 else -((-q.numerator) // q.denominator)
    return t if INT_MIN <= t <= INT_MAX else "Overflow"


def fromint(i):
    return from_fraction(Fraction(i))


def parse(s):
    """strconv.ParseFloat of a decimal text such as -12.5e-3 (correctly rounded)."""
    t = s.strip()
    sign = -1 if t.startswith("-") else 1
    t = t.lstrip("+-")
    mant, _, exp = t.lower().partition("e")
    whole, _, frac = mant.partition(".")
    q = Fraction(int((whole + frac) or "0")) / Fraction(10) ** len(frac) * Fraction(10) ** int(exp or "0")
    return from_fraction(sign * q, negzero=sign < 0)


def show(b):
    if isinstance(b, bool):
        return "true" if b else "false"
    if isinstance(b, int) and not isinstance(b, bool) and b > (1 << 64):
        return str(b)
    return NAN if b == NAN else "%016x" % b


# --- operands -------------------------------------------------------------------------------------

SPECIAL = [0, SIGN, POSINF, NEGINF, NAN, 1, SIGN | 1, FRACMASK, 1 << 52, 0x7FEFFFFFFFFFFFFF,
           0x3FF0000000000000, 0xBFF0000000000000, 0x3FB999999999999A, 0x3FD5555555555555,
           0x4340000000000000, 0x433FFFFFFFFFFFFF, 0x43E0000000000000, 0xC3E0000000000000,
           0x43DFFFFFFFFFFFFF, 0xC3E0000000000001, 0x3FE0000000000000, 0x3FF8000000000000]


def operand(rng):
    r = rng.random()
    if r < 0.25:
        return rng.choice(SPECIAL)
    if r < 0.45:  # near one: exercises the rounding of sums and products
        return rng.choice([0, SIGN]) | (rng.randint(1020, 1026) << 52) | rng.getrandbits(52)
    if r < 0.55:  # subnormals
        return rng.choice([0, SIGN]) | rng.getrandbits(52)
    b = rng.getrandbits(64)
    return NAN if (b & EXPMASK) == EXPMASK and (b & FRACMASK) else b


def host(b):
    return struct.unpack("<d", struct.pack("<Q", b))[0] if b != NAN else float("nan")


def unhost(f):
    return NAN if f != f else struct.unpack("<Q", struct.pack("<d", f))[0]


def selftest(n):
    rng = random.Random(1)
    bad = 0
    fusiondiff = 0
    for k in range(n):
        x, y, z = operand(rng), operand(rng), operand(rng)
        hx, hy, hz = host(x), host(y), host(z)
        pairs = [("add", add(x, y), hx + hy), ("sub", sub(x, y), hx - hy), ("mul", mul(x, y), hx * hy),
                 ("muladd", muladd(x, y, z), hx + hy * hz)]
        try:
            hd = hx / hy
        except ZeroDivisionError:  # Python raises where IEEE gives a value; the oracle decides
            hd = None
        if hd is not None:
            pairs.append(("div", div(x, y), hd))
        for name, o, h in pairs:
            if o != unhost(h):
                bad += 1
                if bad <= 10:
                    print(f"float-oracle: {name} {show(x)} {show(y)} {show(z)}: oracle {show(o)}, host {show(unhost(h))}")
        for name, o, h in [("eq", eq(x, y), hx == hy), ("lt", lt(x, y), hx < hy), ("le", le(x, y), hx <= hy)]:
            if o != h:
                bad += 1
                if bad <= 10:
                    print(f"float-oracle: {name} {show(x)} {show(y)}: oracle {o}, host {h}")
        if fma(y, z, x) != muladd(x, y, z):
            fusiondiff += 1
        if x != NAN and not is_inf(x):  # the host converts like the oracle within range
            t = toint(x)
            if t != "Overflow" and t != int(hx):
                bad += 1
        i = rng.choice([INT_MIN, INT_MAX, 0, -1, (1 << 53) + 1, rng.randint(INT_MIN, INT_MAX)])
        if fromint(i) != unhost(float(i)):
            bad += 1
            if bad <= 10:
                print(f"float-oracle: fromint {i}: oracle {show(fromint(i))}, host {show(unhost(float(i)))}")
        s = "%d.%de%d" % (rng.randint(0, 10 ** 17), rng.randint(0, 10 ** 6), rng.randint(-330, 310))
        if parse(s) != unhost(float(s)):
            bad += 1
            if bad <= 10:
                print(f"float-oracle: parse {s}: oracle {show(parse(s))}, host {show(unhost(float(s)))}")
    if bad:
        sys.exit(f"float-oracle: selftest FAIL: {bad} disagreement(s) in {n} rounds")
    print(f"float-oracle: selftest ok: {n} rounds agree with the host's unfused IEEE arithmetic; "
          f"{fusiondiff} muladd cases round differently when fused (what D188 forbids)")


def cases(seed, n):
    rng = random.Random(seed)
    out = []
    for _ in range(n):
        x, y, z = operand(rng), operand(rng), operand(rng)
        op = rng.choice(["add", "sub", "mul", "div", "neg", "muladd", "fma", "eq", "lt", "le", "toint", "fromint"])
        if op == "neg":
            out.append(f"neg {show(x)} -> {show(neg(x))}")
        elif op in ("muladd", "fma"):
            r = muladd(x, y, z) if op == "muladd" else fma(x, y, z)
            out.append(f"{op} {show(x)} {show(y)} {show(z)} -> {show(r)}")
        elif op == "toint":
            out.append(f"toint {show(x)} -> {toint(x)}")
        elif op == "fromint":
            i = rng.choice([INT_MIN, INT_MAX, (1 << 53) + 1, rng.randint(INT_MIN, INT_MAX)])
            out.append(f"fromint {i} -> {show(fromint(i))}")
        else:
            f = {"add": add, "sub": sub, "mul": mul, "div": div, "eq": eq, "lt": lt, "le": le}[op]
            out.append(f"{op} {show(x)} {show(y)} -> {show(f(x, y))}")
    return out


def main():
    a = sys.argv[1:]
    if a[:1] == ["selftest"]:
        selftest(int(a[1]) if len(a) > 1 else 20000)
    elif a[:1] == ["cases"] and len(a) == 3:
        print("\n".join(cases(int(a[1]), int(a[2]))))
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main()
