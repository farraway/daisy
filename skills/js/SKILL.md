---
name: js
description: Vanilla JavaScript conventions — strict mode, const/let, destructuring, collections, classes, async/await, coercion rules. Use when writing or modifying JavaScript, or when asked about JS style, async patterns, or type handling.
short_description: Vanilla JavaScript conventions and style
---

Derived from a full walkthrough of Piccalilli's *JavaScript for Everyone*
(54 lessons). Follow these unless the existing codebase has established
conventions that conflict.

The through-line is **explicit over implicit**: say what you mean rather than
relying on the language to infer it. Semicolons over ASI, `===` over
coercion, `const` over reassignable bindings, declared fields over emergent
ones. Where the modern standard library has a purpose-built method, use it
over the older idiom it replaced.

## Strict mode

Put `"use strict"` at the top of every script and function; treat it as the
default rather than an option. Use the string literal form (not a template
literal) so older engines ignore it gracefully. Class bodies are already
strict.

## Formatting

- Terminate statements with explicit semicolons — never leave it to
  automatic semicolon insertion.
- Use block statements `{}` with `if`/`else`, even for a single statement.
- Prefer `//` line comments over `/* */` blocks.

## Naming

`camelCase` for variables, functions, properties, and methods. `PascalCase`
for classes and constructor functions — capitalisation is the signal that
`new` is expected. Names should be descriptive; abbreviations like `ret`,
`val`, `tmp` are acceptable only where the scope is trivially small.

## Variables and scope

Default to `const`; use `let` only where reassignment is required. `var` is
function-scoped and hoists, so it has no remaining use — including as a
`for...of` loop binding.

One declaration per line, no comma-separated binding lists. Don't shadow
variables in nested scopes.

## Destructuring

Prefer destructuring over manual index or key access. Alias where needed
(`const { propKey: localName } = obj`), and wrap assignment-pattern object
destructuring in parentheses: `({ a, b } = obj)`.

## Types and coercion

- Compare with `===`. `==` applies coercion rules that surprise readers.
- Check for `NaN` with `Number.isNaN(value)` — `=== NaN` is always false and
  global `isNaN()` coerces first.
- Use `null` for intentional absence; let `undefined` mean implicit absence
  (uninitialised variables, missing properties, no return value).
- Guard with `typeof` before operating on a value of uncertain type.
- Call `Boolean(value)` as a function. `new Boolean()` produces an object,
  which is always truthy.
- Suffix BigInt literals with `n` (`9007199254740993n`), and keep BigInt and
  Number out of the same arithmetic.

## Strings

Prefer template literals for interpolation and multiline strings. Where `+`
mixes strings and numbers, parenthesise to control evaluation order:
`"Total: " + (a + b)`.

## Arrays

Use array literals `[]` rather than `new Array()`. Avoid sparse arrays —
behaviour across array methods is inconsistent. Reach for `.at(-1)` to index
from the end, and `Array.from()` to convert array-likes (`NodeList`,
`arguments`, `HTMLCollection`).

## Collections

Use `Set` for unique values, and `Map` when keys aren't strings or insertion
order matters — accessed with `.get()`/`.set()`, not dot or bracket
notation. Use `WeakSet`/`WeakMap` to track objects without preventing
garbage collection.

## Objects

- Use object literals `{}` rather than `new Object()`.
- `Object.hasOwn(obj, key)` for own-property checks, in place of
  `obj.hasOwnProperty(key)`; `Object.getPrototypeOf(obj)` to read a
  prototype, in place of `obj.__proto__`.
- Method shorthand `{ method() {} }`, not `{ method: function() {} }`.
- Getters and setters for computed or validated properties.
- Computed property names `{ [expr]: value }` for dynamic keys.
- Dot notation for identifier-safe keys; brackets for dynamic or special
  ones.

## Spread and copying

Spread for shallow copies (`{ ...obj }`, `[ ...arr ]`), `structuredClone(obj)`
for deep copies where nested objects must be independent. Objects aren't
iterable, so they don't spread into arrays.

## Symbols

Use Symbols for collision-free property keys. Store the reference in a
variable — an identical `Symbol()` call produces a different symbol. Pass a
descriptive string: `Symbol("internalId")`.

## Functions

- Define functions before calling them rather than relying on hoisting.
- Use named function expressions for better stack traces.
- Arrow functions have no own `this` or `arguments` — use them when you want
  lexical `this`, and not as constructors.
- `?.()` for conditional invocation; `typeof x === "function"` for a
  stronger callable check.
- Rest parameters (`...args`) in place of the `arguments` object.
- Default parameter values for optional arguments.

## `this`

`this` is determined at call time, not definition time — except in arrow
functions, which capture it lexically from the enclosing scope. Bind
explicitly with `.call()`, `.apply()`, or `.bind()`.

## Iteration

`for...of` for iterables; object literals aren't iterable. Use
`Object.keys()`, `Object.values()`, or `Object.entries()` for object own
properties. `for...in` walks inherited enumerable properties too — filter
with `Object.hasOwn()`. Iterators are single-use; iterables can be walked
repeatedly.

## Generators

`function*` returns an iterator object; call `.next()` on that same instance
to step through. A `return` value inside a generator is excluded from
`for...of` and spread.

## Classes

- Include an explicit `constructor`.
- Declare instance properties at the top using public field syntax.
- Prototype methods for shared behaviour.
- Call `super()` before referencing `this` in a subclass constructor.
- Private fields (`#field`) are neither inherited nor accessible in
  subclasses.

## Async

Handle asynchronous work with `async`/`await` in preference to raw Promises,
and always handle rejections.

- Use Promises for genuinely asynchronous work; don't wrap synchronous code
  in one.
- Chain `.then()` and `.catch()` separately rather than passing both
  callbacks to `.then()`.
- `.finally()` for cleanup that runs either way.
- `Promise.all()` with `await` for independent concurrent operations,
  destructured: `const [a, b] = await Promise.all([p1, p2])`. Sequential
  `await` where operations depend on each other.
- `Promise.allSettled()` when you need every result regardless of outcome.
