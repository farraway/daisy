---
name: tdd
description: Test-driven development — the red/green loop, choosing seams, when to mock, and the anti-patterns that produce tests worth deleting. Use when writing tests, building a feature or fixing a bug test-first, when an implementation step has a testable seam, or when the user mentions red-green-refactor or integration tests.
short_description: Test-first development: seams, mocks, anti-patterns
---

TDD is the **red → green** loop. This reference is what makes that loop
produce tests worth keeping: what a good test is, where tests go, the
anti-patterns, and the rules of the loop. Every section applies on every
cycle — consult them before and during, not after.

When exploring the codebase, read `CONTEXT.md` if it exists so test names and
interface vocabulary match the project's domain language, and respect ADRs
in the area you're touching.

## What a good test is

Tests verify behaviour through public interfaces, not implementation
details. Code can change entirely; tests shouldn't. A good test reads like a
specification — "user can checkout with valid cart" tells you exactly what
capability exists — and survives refactors because it doesn't care about
internal structure.

**Good — integration-style, through real interfaces:**

```typescript
test("user can checkout with valid cart", async () => {
  const cart = createCart();
  cart.add(product);
  const result = await checkout(cart, paymentMethod);
  expect(result.status).toBe("confirmed");
});
```

It tests behaviour callers care about, uses only the public API, survives
internal refactors, describes WHAT rather than HOW, and makes one logical
assertion.

**Bad — coupled to implementation:**

```typescript
test("checkout calls paymentService.process", async () => {
  const mockPayment = jest.mock(paymentService);
  await checkout(cart, payment);
  expect(mockPayment.process).toHaveBeenCalledWith(cart.total);
});
```

Red flags: mocking internal collaborators, testing private methods,
asserting on call counts or order, a test that breaks when you refactor
without changing behaviour, a name describing HOW, and verification through
a side channel rather than the interface:

```typescript
// BAD: bypasses interface to verify
test("createUser saves to database", async () => {
  await createUser({ name: "Alice" });
  const row = await db.query("SELECT * FROM users WHERE name = ?", ["Alice"]);
  expect(row).toBeDefined();
});

// GOOD: verifies through interface
test("createUser makes user retrievable", async () => {
  const user = await createUser({ name: "Alice" });
  const retrieved = await getUser(user.id);
  expect(retrieved.name).toBe("Alice");
});
```

**Tautological** tests restate the implementation in the expected value, so
they pass by construction and can never disagree with the code:

```typescript
// BAD: expected value recomputed the way the code computes it
test("calculateTotal sums line items", () => {
  const items = [{ price: 10 }, { price: 5 }];
  const expected = items.reduce((sum, i) => sum + i.price, 0);
  expect(calculateTotal(items)).toBe(expected);
});

// GOOD: expected value is an independent, known literal
test("calculateTotal sums line items", () => {
  expect(calculateTotal([{ price: 10 }, { price: 5 }])).toBe(15);
});
```

Expected values come from an independent source of truth — a known-good
literal, a worked example, the spec.

## Seams — where tests go

A **seam** is the public boundary you test at: the interface where you
observe behaviour without reaching inside. Tests live at seams, never
against internals.

**Test only at pre-agreed seams.** Before writing any test, write down the
seams under test and confirm them with the user. No test is written at an
unconfirmed seam. You can't test everything — agreeing the seams up front is
how testing effort lands on critical paths and complex logic instead of
every edge case.

Ask: "What's the public interface, and which seams should we test?"

## When to mock

Mock at **system boundaries** only: external APIs (payment, email), databases
(sometimes — prefer a test DB), time and randomness, the file system
(sometimes). Your own classes, internal collaborators, and anything else you
control get exercised for real.

**Designing for mockability at those boundaries:**

1. **Dependency injection** — pass external dependencies in rather than
   constructing them internally:

```typescript
// Easy to mock
function processPayment(order, paymentClient) {
  return paymentClient.charge(order.total);
}

// Hard to mock
function processPayment(order) {
  const client = new StripeClient(process.env.STRIPE_KEY);
  return client.charge(order.total);
}
```

2. **SDK-style interfaces over generic fetchers** — one specific function per
   external operation:

```typescript
// GOOD: each function is independently mockable
const api = {
  getUser: (id) => fetch(`/users/${id}`),
  getOrders: (userId) => fetch(`/users/${userId}/orders`),
  createOrder: (data) => fetch('/orders', { method: 'POST', body: data }),
};

// BAD: mocking requires conditional logic inside the mock
const api = {
  fetch: (endpoint, options) => fetch(endpoint, options),
};
```

Each mock then returns one specific shape, test setup carries no conditional
logic, it's visible which endpoints a test exercises, and types hold per
endpoint.

## Anti-patterns

- **Implementation-coupled** — mocks internal collaborators, tests private
  methods, or verifies through a side channel. The tell: the test breaks
  when you refactor but behaviour hasn't changed.
- **Tautological** — the assertion recomputes the expected value the way the
  code does.
- **Horizontal slicing** — all tests first, then all implementation. Bulk
  tests verify *imagined* behaviour: you test the shape of things rather
  than user-facing behaviour, the tests go insensitive to real changes, and
  you commit to test structure before understanding the implementation. Work
  in **vertical slices** instead — one test, one implementation, repeat,
  each test a **tracer bullet** that responds to what the last cycle taught
  you.

## Rules of the loop

- **Red before green.** Write the failing test first, then only enough code
  to pass it. Don't anticipate future tests or add speculative features.
- **One slice at a time.** One seam, one test, one minimal implementation
  per cycle.
- **Refactoring is not part of the loop.** It belongs to the review stage,
  not the red → green cycle.
