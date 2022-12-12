import { action } from '@storybook/addon-actions';
import { useArgs } from '@storybook/client-api';
import { ComponentMeta, ComponentStory } from '@storybook/react';
import React from 'react';

import { TimeZonePicker } from '@grafana/ui';

import { withCenteredStory } from '../../utils/storybook/withCenteredStory';

const meta: ComponentMeta<typeof TimeZonePicker> = {
  title: 'Pickers and Editors/TimePickers/TimeZonePicker',
  component: TimeZonePicker,
  decorators: [withCenteredStory],
  parameters: {
    controls: {
      exclude: ['inputId', 'onChange', 'onBlur'],
    },
  },
  args: {
    value: 'Europe/Stockholm',
  },
  argTypes: {
    includeInternal: {
      control: {
        type: 'boolean',
      },
    },
  },
};

export const Basic: ComponentStory<typeof TimeZonePicker> = (args) => {
  const [, updateArgs] = useArgs();
  return (
    <TimeZonePicker
      {...args}
      onChange={(newValue) => {
        action('on selected')(newValue);
        updateArgs({ value: newValue });
      }}
      onBlur={action('onBlur')}
    />
  );
};

export default meta;
