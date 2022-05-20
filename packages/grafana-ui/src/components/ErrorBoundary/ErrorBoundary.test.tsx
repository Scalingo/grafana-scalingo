import { captureException } from '@sentry/browser';
import { render, screen } from '@testing-library/react';
import React, { FC } from 'react';

import { ErrorBoundary } from './ErrorBoundary';

jest.mock('@sentry/browser');

const ErrorThrower: FC<{ error: Error }> = ({ error }) => {
  throw error;
};

// According to this issue https://github.com/facebook/react/issues/15069 componentDidCatch logs errors to console.error unconditionally.
// Let's make sure we don't output that to console.error in the tests.
let consoleSpy: jest.SpyInstance;
describe('ErrorBoundary', () => {
  beforeEach(() => {
    consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    consoleSpy.mockRestore();
  });

  it('should catch error and report it to sentry, including react component stack in context', async () => {
    const problem = new Error('things went terribly wrong');
    render(
      <ErrorBoundary>
        {({ error }) => {
          if (!error) {
            return <ErrorThrower error={problem} />;
          } else {
            return <p>{error.message}</p>;
          }
        }}
      </ErrorBoundary>
    );

    await screen.findByText(problem.message);
    expect(captureException).toHaveBeenCalledTimes(1);
    const [error, context] = (captureException as jest.Mock).mock.calls[0];
    expect(error).toBe(problem);
    expect(context).toHaveProperty('contexts');
    expect(context.contexts).toHaveProperty('react');
    expect(context.contexts.react).toHaveProperty('componentStack');
    expect(context.contexts.react.componentStack).toMatch(/^\s+at ErrorThrower (.*)\s+at ErrorBoundary (.*)\s*$/);
  });

  it('should recover when when recover props change', async () => {
    const problem = new Error('things went terribly wrong');
    let renderCount = 0;

    const { rerender } = render(
      <ErrorBoundary dependencies={[1, 2]}>
        {({ error }) => {
          if (!error) {
            renderCount += 1;
            return <ErrorThrower error={problem} />;
          } else {
            return <p>{error.message}</p>;
          }
        }}
      </ErrorBoundary>
    );

    await screen.findByText(problem.message);

    rerender(
      <ErrorBoundary dependencies={[1, 3]}>
        {({ error }) => {
          if (!error) {
            renderCount += 1;
            return <ErrorThrower error={problem} />;
          } else {
            return <p>{error.message}</p>;
          }
        }}
      </ErrorBoundary>
    );

    expect(renderCount).toBe(2);
  });
});
