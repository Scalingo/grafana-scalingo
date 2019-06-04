import React from 'react';

import { storiesOf } from '@storybook/react';
import TableInputCSV from './TableInputCSV';
import { action } from '@storybook/addon-actions';
import { SeriesData } from '../../types/data';
import { withCenteredStory } from '../../utils/storybook/withCenteredStory';

const TableInputStories = storiesOf('UI/Table/Input', module);

TableInputStories.addDecorator(withCenteredStory);

TableInputStories.add('default', () => {
  return (
    <TableInputCSV
      width={400}
      height={'90vh'}
      text={'a,b,c\n1,2,3'}
      onSeriesParsed={(data: SeriesData[], text: string) => {
        console.log('Data', data, text);
        action('Data')(data, text);
      }}
    />
  );
});
