import React from 'react';

import { withCenteredStory } from '../../utils/storybook/withCenteredStory';
import { UnitPicker } from './UnitPicker';
import mdx from './UnitPicker.mdx';

export default {
  title: 'UI/UnitPicker',
  component: UnitPicker,
  decorators: [withCenteredStory],
  parameters: {
    docs: mdx,
  },
};

export const simple = () => <UnitPicker onChange={val => console.log(val)} />;
