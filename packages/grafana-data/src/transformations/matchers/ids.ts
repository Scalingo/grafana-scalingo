// This needs to be in its own file to avoid circular references

// Builtin Predicates
// not using 'any' and 'never' since they are reservered keywords
export enum MatcherID {
  anyMatch = 'anyMatch', // checks children
  allMatch = 'allMatch', // checks children
  invertMatch = 'invertMatch', // checks child
  alwaysMatch = 'alwaysMatch',
  neverMatch = 'neverMatch',
}

export enum FieldMatcherID {
  // Specific Types
  numeric = 'numeric',
  time = 'time',

  // With arguments
  byType = 'byType',
  byName = 'byName',
  // byIndex = 'byIndex',
  // byLabel = 'byLabel',
}

/**
 * Field name matchers
 */
export enum FrameMatcherID {
  byName = 'byName',
  byRefId = 'byRefId',
  byIndex = 'byIndex',
  byLabel = 'byLabel',
}
