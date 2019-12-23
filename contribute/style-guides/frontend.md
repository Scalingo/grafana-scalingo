# Frontend Style Guide

Generally we follow the Airbnb [React Style Guide](https://github.com/airbnb/javascript/tree/master/react).

## Table of Contents

- [Frontend Style Guide](#frontend-style-guide)
  - [Table of Contents](#table-of-contents)
  - [Basic rules](#basic-rules)
  - [Organization](#organization)
  - [Props](#props)
  - [State management](#state-management)

## Basic rules

- Try to keep files small and focused.
- Break large components up into sub-components.

## Organization

- Components and types that needs to be used by external plugins needs to go into @grafana/ui
- Components should get their own folder under features/xxx/components
  - Sub components can live in that component folders, so small component do not need their own folder
  - Place test next to their component file (same dir)
  - Component sass should live in the same folder as component code
- State logic & domain models should live in features/xxx/state
- Containers (pages) can live in feature root features/xxx
  - up for debate?

## Props

- Name callback props and handlers with an "on" prefix.

```tsx
// good
onChange = () => {

};

render() {
  return (
    <MyComponent onChange={this.onChange} />
  );
}

// bad
handleChange = () => {

};

render() {
  return (
    <MyComponent changed={this.handleChange} />
  );
}
```

- React Component definitions

```jsx
// good
export class YourClass extends PureComponent<{},{}> { ... }

// bad
export class YourClass extends PureComponent { ... }
```

- React Component constructor

```typescript
// good
constructor(props:Props) {...}

// bad
constructor(props) {...}
```

- React Component defaultProps

```typescript
// good
static defaultProps: Partial<Props> = { ... }

// bad
static defaultProps = { ... }
```

## State management

- Don't mutate state in reducers or thunks.
- Use helpers `actionCreatorFactory` and `reducerFactory` instead of traditional `switch statement` reducers in Redux. See [Redux framework](redux.md) for more details.
- Use `reducerTester` to test reducers. See [Redux framework](redux.md) for more details.
- Use state selectors to access state instead of accessing state directly.
