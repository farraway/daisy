---
name: js
description: Vanilla JavaScript conventions and patterns. Use when writing or modifying JavaScript files, discussing JS conventions/patterns/code style, working on web projects using vanilla JS, or asked about async patterns, classes, or type handling in JS.
short_description: Vanilla JavaScript conventions and patterns
---

# JavaScript Best Practices

Derived from a full walkthrough of Piccalilli's JavaScript for Everyone course (54 lessons). Follow these unless the existing codebase has established conventions that conflict.

## Rules

1. **Use `"use strict"` at the top of every script.** Treat strict mode as the default, not an option.
2. **Always use explicit semicolons.** Never rely on ASI.
3. **Use block statements `{}` with `if`/`else`.** Even for single-statement bodies.
4. **`camelCase` for variables/functions, `PascalCase` for classes.** No abbreviations unless scope is trivially small.
5. **Prefer `const` by default; use `let` only when reassignment is needed; never use `var`.**
6. **Handle async with `async`/`await`.** Prefer it over raw Promises for clarity; always handle rejections.
7. **Never use `==`; always use `===` for equality.**

## Strict Mode

- Use `"use strict"` at the top of scripts and functions. Treat strict mode as the default.
- Use the string literal form (`"use strict"`, not a template literal) so older engines ignore it gracefully.

## Semicolons and Formatting

- Use explicit semicolons. Never rely on ASI.
- Use block statements `{}` with `if`/`else`, even for single statements.
- Prefer `//` line comments over `/* */` block comments.

## Naming

- `camelCase` for variables, functions, properties, and methods.
- `PascalCase` for classes and constructor functions.
- Use descriptive names. Avoid abbreviations like `ret`, `val`, `tmp` unless the scope is trivially small.

## Variables and Scope

- Default to `const`. Use `let` only when reassignment is required.
- Never use `var`.
- One declaration per line. No comma-separated binding lists.
- Do not shadow variables in nested scopes.
- Use `const` or `let` in `for...of` loop variables, never `var`.

## Destructuring

- Prefer destructuring over manual index/key access for arrays and objects.
- Use aliases when needed: `const { propKey: localName } = obj`.
- Wrap assignment-pattern object destructuring in parentheses: `({ a, b } = obj)`.

## Types and Coercion

- Use `Number.isNaN(value)` to check for `NaN`. Never use `=== NaN` or global `isNaN()`.
- Use `null` for intentional absence. Let `undefined` represent implicit absence (uninitialised variables, missing properties, no return value).
- Use `typeof` guards before operations when the type is uncertain.
- Use `Boolean(value)` as a function call, never `new Boolean()`.
- Use the `n` suffix for BigInt literals (`9007199254740993n`). Do not mix BigInt and Number in arithmetic.

## Strings

- Prefer template literals for interpolation and multiline strings.
- Use string literals (not template literals) for `"use strict"`.
- Use parentheses to control evaluation order when `+` mixes strings and numbers: `"Total: " + (a + b)`.

## Arrays

- Use array literal syntax `[]`. Never use `new Array()`.
- Avoid sparse arrays (empty slots). Behavior is inconsistent across array methods.
- Use `.at(-1)` for accessing elements from the end (ES2022).
- Use `Array.from()` to convert array-likes (`NodeList`, `arguments`, `HTMLCollection`).

## Collections

- Use `Set` for unique values. Use `Map` when keys are not strings or insertion order matters.
- Access Map entries with `.get()`/`.set()`, not dot or bracket notation.
- Use `WeakSet`/`WeakMap` when tracking objects without preventing garbage collection.

## Objects

- Use object literals `{}`. Never use `new Object()`.
- Use `Object.hasOwn(obj, key)` instead of `obj.hasOwnProperty(key)`.
- Use `Object.getPrototypeOf(obj)` instead of `obj.__proto__`.
- Use method shorthand: `{ method() {} }` not `{ method: function() {} }`.
- Use getters and setters for computed or validated properties.
- Use computed property names `{ [expr]: value }` for dynamic keys.
- Prefer dot notation for identifier-safe keys; bracket notation for dynamic or special keys.

## Spread and Copying

- Use spread for shallow copies: `{ ...obj }`, `[ ...arr ]`.
- Use `structuredClone(obj)` for deep copies when nested objects must be independent.
- Objects are not iterable. Do not spread objects into arrays.

## Symbols

- Use Symbols for collision-free property keys.
- Always store a Symbol reference in a variable; you cannot recreate the same Symbol.
- Pass a descriptive string: `Symbol("internalId")`.

## Functions

- Do not rely on hoisting. Define functions before calling them.
- Use named function expressions for better stack traces.
- Arrow functions have no own `this` or `arguments`. Use them when you want lexical `this`.
- Use `?.()` for conditional invocation. Use `typeof x === "function"` for stronger callable checks.
- Use rest parameters (`...args`) instead of the `arguments` object.
- Use default parameter values for optional arguments.
- Capitalise constructor function names to signal `new` usage.
- Do not use arrow functions as constructors.

## Generators

- `function*` returns an iterator object. Call `.next()` on the same instance to step through values.
- A `return` value inside a generator is not included in `for...of` or spread.

## `this`

- `this` is determined at call time, not definition time (except arrow functions).
- Arrow functions capture `this` lexically from the enclosing scope.
- Use `.call()`, `.apply()`, or `.bind()` for explicit binding.

## Iteration

- Use `for...of` for iterables. Object literals are not iterable.
- Use `Object.keys()`, `Object.values()`, or `Object.entries()` to iterate object own properties.
- `for...in` includes inherited enumerable properties. Filter with `Object.hasOwn()` if needed.
- Treat iterators as single-use. Iterables can be iterated multiple times; iterators cannot.

## Classes

- All class bodies run in strict mode.
- Include an explicit `constructor`.
- Declare instance properties at the top of the class using public field syntax (ES2022+).
- Start class identifiers with a capital letter.
- Use prototype methods for shared behavior.
- In subclass constructors, call `super()` before referencing `this`.
- Private fields (`#field`) are not inherited or accessible in subclasses.

## Async

- Use Promises for genuinely asynchronous work. Do not wrap synchronous code in Promises.
- Chain with `.then()` and `.catch()` separately. Do not pass both callbacks to `.then()`.
- Use `.finally()` for cleanup that runs regardless of outcome.
- Use `Promise.all()` with `await` for independent concurrent operations. Use sequential `await` when operations depend on each other.
- Use `Promise.allSettled()` when you need results from all Promises regardless of success/failure.
- Use array destructuring with `Promise.all()`: `const [a, b] = await Promise.all([p1, p2])`.
