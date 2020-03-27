import { text } from '@storybook/addon-knobs';
import { withCenteredStory } from '../../utils/storybook/withCenteredStory';
import { ValuePicker } from './ValuePicker';
import React from 'react';
import { generateOptions } from '../Forms/Select/Select.story';

export default {
  title: 'General/ValuePicker',
  component: ValuePicker,
  decorators: [withCenteredStory],
};

const options = generateOptions();

export const simple = () => {
  const label = text('Label', 'Pick an option');
  return (
    <div style={{ width: '200px' }}>
      <ValuePicker options={options} label={label} onChange={v => console.log(v)} />
    </div>
  );
};
