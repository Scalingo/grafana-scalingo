import { Story } from '@storybook/react';
import React, { useState } from 'react';

import { withCenteredStory } from '@grafana/ui/src/utils/storybook/withCenteredStory';

import { getAvailableIcons } from '../../types';
import { HorizontalGroup } from '../Layout/Layout';

import { FilterPill, FilterPillProps } from './FilterPill';
import mdx from './FilterPill.mdx';

export default {
  title: 'General/FilterPill',
  component: FilterPill,
  decorators: [withCenteredStory],
  argTypes: {
    icon: { control: { type: 'select', options: getAvailableIcons() } },
    onClick: { action: 'Pill Clicked' },
  },
  parameters: {
    docs: {
      page: mdx,
    },
  },
};

export const Basic: Story<FilterPillProps> = (args) => {
  return <FilterPill {...args} />;
};

export const Example = () => {
  const [selected, setSelected] = useState('Stockholm');
  const elements = ['Singapore', 'Paris', 'Stockholm', 'New York', 'London'];

  return (
    <HorizontalGroup>
      {elements.map((item) => (
        <FilterPill key={item} label={item} selected={item === selected} onClick={() => setSelected(item)} />
      ))}
    </HorizontalGroup>
  );
};

Basic.args = {
  selected: false,
  label: 'Test',
  icon: undefined,
};
