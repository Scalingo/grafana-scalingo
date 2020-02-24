import { Vector } from '../types/vector';
import { DataFrame } from '../types/dataFrame';
import { DisplayProcessor } from '../types';

/**
 * This abstraction will present the contents of a DataFrame as if
 * it were a well typed javascript object Vector.
 *
 * NOTE: The contents of the object returned from `view.get(index)`
 * are optimized for use in a loop.  All calls return the same object
 * but the index has changed.
 *
 * For example, the three objects:
 *   const first = view.get(0);
 *   const second = view.get(1);
 *   const third = view.get(2);
 * will point to the contents at index 2
 *
 * If you need three different objects, consider something like:
 *   const first = { ... view.get(0) };
 *   const second = { ... view.get(1) };
 *   const third = { ... view.get(2) };
 */
export class DataFrameView<T = any> implements Vector<T> {
  private index = 0;
  private obj: T;

  constructor(private data: DataFrame) {
    const obj = ({} as unknown) as T;

    for (let i = 0; i < data.fields.length; i++) {
      const field = data.fields[i];
      const getter = () => field.values.get(this.index);

      if (!(obj as any).hasOwnProperty(field.name)) {
        Object.defineProperty(obj, field.name, {
          enumerable: true, // Shows up as enumerable property
          get: getter,
        });
      }

      Object.defineProperty(obj, i, {
        enumerable: false, // Don't enumerate array index
        get: getter,
      });
    }

    this.obj = obj;
  }

  get dataFrame() {
    return this.data;
  }

  get length() {
    return this.data.length;
  }

  getFieldDisplayProcessor(colIndex: number): DisplayProcessor | null {
    if (!this.dataFrame || !this.dataFrame.fields) {
      return null;
    }

    const field = this.dataFrame.fields[colIndex];

    if (!field || !field.display) {
      return null;
    }

    return field.display;
  }

  get(idx: number) {
    this.index = idx;
    return this.obj;
  }

  toArray(): T[] {
    return new Array(this.data.length).fill(0).map((_, i) => ({ ...this.get(i) }));
  }

  toJSON(): T[] {
    return this.toArray();
  }

  forEachRow(iterator: (row: T) => void) {
    for (let i = 0; i < this.data.length; i++) {
      iterator(this.get(i));
    }
  }

  map<V>(iterator: (item: T, index: number) => V) {
    const acc: V[] = [];
    for (let i = 0; i < this.data.length; i++) {
      acc.push(iterator(this.get(i), i));
    }
    return acc;
  }
}
