# How Pint works

pint-go follows the same conceptual model as [Pint](https://github.com/hgrecco/pint). This page covers definition lines, how conversion walks them, and the other pieces that live in those files (contexts, temperature offsets, groups, systems).

API calls, concurrency, and typeahead are in [go.md](go.md).

Pint keeps a **web of names**. Each name equals some amount of something else. There is no table of every pair (`min` → `hour`, `atm` → `psi`, ...). To convert, you walk both names down to the same foundation and multiply the numbers along the way.

A **quantity** is a magnitude plus a unit. Examples: `3 atm`, `20 degC`, `5.2 kPa`. The **registry** is the web those names live in.

## Definition lines

Pint ships two English files, `default_en.txt` and `constants_en.txt`. The first file `@import`s the second. You can load extra product lines on top in the same syntax. The bundled files stay as Pint wrote them.

A typical line looks like this:

```text
minute = 60 * second = min
```

Left of the first `=` is the canonical name. The next field is what it equals. Further fields are a symbol and aliases (`min` is another spelling of `minute`). If you want aliases but no symbol, write `_` in the symbol slot.

A line gives a **name** to a **size**. The syntax does not mark that size as a unit or as a constant. The shapes below are the ones you will run into.

**A starting point for a kind of measurement**

```text
meter = [length] = m = metre
second = [time] = s
gram = [mass] = g
```

`meter` is how length is counted. `[length]` is a **dimension**. You do not type `[length]` as the unit on a quantity. Two values convert only if they reduce to the same kind: `[length]`, `[time]`, pressure as `[mass] / [length] / [time] ** 2`, and so on. Derived dimensions use the same writing: `[density] = [mass] / [volume]`. Some bases are dimensionless: `radian = []`, `count = []`.

**A multiple of something already named**

```text
kilopascal = 1000 * pascal
standard_atmosphere = 1.01325e5 Pa = atm = atmosphere
speed_of_light = 299792458 m/s = c
percent = 0.01
angstrom = 1e-10 * meter = Å
```

Each of these is still a name, a scale, and what it is made of. `atm` has the dimension of pressure, `c` of speed, `percent` of a dimensionless ratio. Pint stores them the same way. People put `atm` on a measurement. `planck_constant` stays in the web so other definitions and context formulas can refer to it. Which file the line came from does not tell you which of those roles it has. `constants_en.txt` is a NIST-oriented dump, and it also contains conventional units such as `atm`.

**Another spelling, or a prefix**

`metre` is `meter`. A prefix is a scale glued onto a name:

```text
kilo- = 1e3 = k-
```

`kPa` parses as kilopascal. Pint does not need a separate catalog row for every prefixed combination. Binary prefixes (`kibi-` and the rest) work the same way.

`@group`, `@context`, and `@system` blocks live in the same files. Groups collect units (US customary lengths, for example). Contexts are conversion bridges between different dimensions. Systems pick preferred bases (SI, cgs, US) when you ask to reduce a quantity.

## Conversion

Take `3 atm` → `kPa`.

1. Parse the words into a number and a named size: `3` of `standard_atmosphere` (`atm` is an alias).
2. Follow each name's "equals" until you hit starting points (`meter`, `second`, `kilogram`, ...). Multiply the scales as you go. `atm` becomes 101325 pascal, then that many kilograms, meters, and seconds. `kPa` is 1000 pascal: same foundation, different multiplier.
3. If both walks landed on the same dimension (here, pressure), the ratio of the multipliers is the conversion: `3 × 101325 / 1000` = 303.975 kPa.
4. If the dimensions differ, conversion fails unless a context is in play (below).

There is no special `atm` → `psi` rule. `torr = atm / 760` is another rung on the same ladder. `light_year = speed_of_light * julian_year` uses a named constant as a factor in the same way.

Compound expressions are several names and operators. `meter / second` reduces each name, then combines the dimensions into velocity.

You can ask whether two units would land on the same dimension (compatibility) or go ahead and apply the factor (conversion).

Aliases collapse to a canonical name. `degC` is `degree_Celsius`. Persist the canonical string.

Short symbols can parse as physical constants (`c` is speed of light, `k` is Boltzmann) because those names sit in the same web the engine needs for definitions and context formulas. A unit picker usually offers `atm`. It usually should not offer `planck_constant`.

## Contexts

The default conversion is same-dimension only. Length to frequency is a different question: physics relates them (`speed_of_light / wavelength`). An `@context` block is a formula that hops between dimensions. Those formulas often use a named constant (`planck_constant` for frequency ↔ energy).

The registry loads those graphs and leaves them **off** until you enable them for a conversion. Parameters are quantities. Chemistry `mw` is `18 g/mol`, not a bare `18`. Spectroscopy `n` (index of refraction) is dimensionless.

Built-in contexts include `spectroscopy` (`sp`: nm ↔ THz ↔ energy), `boltzmann` (K ↔ J), `energy` (J ↔ kg, J ↔ J/mol), `chemistry` (`chem`: mol ↔ g given `mw`), plus `textile`, `Gaussian`, and `ESU`.

How pint-go binds those for a request, including scope versus process-wide enable, is in [go.md](go.md) and [conversion-profiles.md](conversion-profiles.md).

## Temperature, logs, offsets

Most units are multiplicative. 2 km is twice 1 km. Temperature scales have a zero that is not "no temperature," so 20 °C is a reading on a scale that has an offset. The definition carries that offset (`degree_Celsius = kelvin; offset: 273.15`). Pint also synthesizes `delta_degC` for an interval: a change of 5 °C versus a reading of 20 °C.

```text
20 degC        → degF         = 68   (absolute)
5  delta_degC  → delta_degF   = 9    (interval)
```

Log units (`dB`, `octave`, `decade`) store a log base and factor instead of a linear scale. They live in the same registry. The converter for those names is logarithmic rather than a simple multiply.

## Groups and systems

A group is a named collection of units (USCS lengths, Troy mass). A system (SI, mks, cgs, US, imperial, ...) says which base names to prefer when reducing "to SI" or "to US." That preference is for display and reduction. Conversion still walks the definition web.
