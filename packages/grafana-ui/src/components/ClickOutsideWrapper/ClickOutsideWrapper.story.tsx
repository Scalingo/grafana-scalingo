import { action } from '@storybook/addon-actions';
import React from 'react';

import { withCenteredStory } from '../../utils/storybook/withCenteredStory';

import { ClickOutsideWrapper } from './ClickOutsideWrapper';
import mdx from './ClickOutsideWrapper.mdx';

export default {
  title: 'Layout/ClickOutsideWrapper',
  component: ClickOutsideWrapper,
  decorators: [withCenteredStory],
  parameters: {
    docs: {
      page: mdx,
    },
  },
};

export const basic = () => {
  return (
    <ClickOutsideWrapper onClick={action('Clicked outside')}>
      <div style={{ width: '300px', border: '1px solid grey', padding: '20px' }}>Container</div>
    </ClickOutsideWrapper>
  );
};
