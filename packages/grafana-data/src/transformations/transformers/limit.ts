import { map } from 'rxjs/operators';

import { DataTransformerInfo } from '../../types';
import { ArrayVector } from '../../vector/ArrayVector';

import { DataTransformerID } from './ids';

export interface LimitTransformerOptions {
  limitField?: number;
}

const DEFAULT_LIMIT_FIELD = 10;

export const limitTransformer: DataTransformerInfo<LimitTransformerOptions> = {
  id: DataTransformerID.limit,
  name: 'Limit',
  description: 'Limit the number of items to the top N',
  defaultOptions: {
    limitField: DEFAULT_LIMIT_FIELD,
  },

  operator: (options) => (source) =>
    source.pipe(
      map((data) => {
        const limitFieldMatch = options.limitField || DEFAULT_LIMIT_FIELD;
        return data.map((frame) => {
          if (frame.length > limitFieldMatch) {
            return {
              ...frame,
              fields: frame.fields.map((f) => {
                const vals = f.values.toArray();
                return {
                  ...f,
                  values: new ArrayVector(vals.slice(0, limitFieldMatch)),
                };
              }),
              length: limitFieldMatch,
            };
          }

          return frame;
        });
      })
    ),
};
