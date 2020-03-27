import { variableAdapters } from '../adapters';
import { updateCustomVariableOptions } from './actions';
import { createCustomVariableAdapter } from './adapter';
import { reduxTester } from '../../../../test/core/redux/reduxTester';
import { getTemplatingRootReducer } from '../state/helpers';
import { VariableOption, VariableHide, CustomVariableModel } from '../variable';
import { toVariablePayload } from '../state/types';
import { setCurrentVariableValue } from '../state/sharedReducer';
import { initDashboardTemplating } from '../state/actions';
import { TemplatingState } from '../state/reducers';
import { createCustomOptionsFromQuery } from './reducer';

describe('custom actions', () => {
  variableAdapters.set('custom', createCustomVariableAdapter());

  describe('when updateCustomVariableOptions is dispatched', () => {
    it('then correct actions are dispatched', async () => {
      const option: VariableOption = {
        value: 'A',
        text: 'A',
        selected: false,
      };

      const variable: CustomVariableModel = {
        type: 'custom',
        uuid: '0',
        global: false,
        current: {
          value: '',
          text: '',
          selected: false,
        },
        options: [
          {
            text: 'A',
            value: 'A',
            selected: false,
          },
          {
            text: 'B',
            value: 'B',
            selected: false,
          },
        ],
        query: 'A,B',
        name: 'Custom',
        label: '',
        hide: VariableHide.dontHide,
        skipUrlSync: false,
        index: 0,
        multi: true,
        includeAll: false,
      };

      const tester = await reduxTester<{ templating: TemplatingState }>()
        .givenRootReducer(getTemplatingRootReducer())
        .whenActionIsDispatched(initDashboardTemplating([variable]))
        .whenAsyncActionIsDispatched(updateCustomVariableOptions(toVariablePayload(variable)), true);

      tester.thenDispatchedActionPredicateShouldEqual(actions => {
        const [createAction, setCurrentAction] = actions;
        const expectedNumberOfActions = 2;

        expect(createAction).toEqual(createCustomOptionsFromQuery(toVariablePayload(variable)));
        expect(setCurrentAction).toEqual(setCurrentVariableValue(toVariablePayload(variable, { option })));
        return actions.length === expectedNumberOfActions;
      });
    });
  });
});
