import { Task, TaskRunner } from './task';
import fs from 'fs';

// @ts-ignore
import execa = require('execa');
import path = require('path');
import glob = require('glob');
import { Linter, Configuration, RuleFailure } from 'tslint';
import * as prettier from 'prettier';

import { useSpinner } from '../utils/useSpinner';
import { testPlugin } from './plugin/tests';
import { bundlePlugin as bundleFn, PluginBundleOptions } from './plugin/bundle';
interface PluginBuildOptions {
  coverage: boolean;
}
interface Fixable {
  fix?: boolean;
}

export const bundlePlugin = useSpinner<PluginBundleOptions>('Compiling...', async options => await bundleFn(options));

// @ts-ignore
export const clean = useSpinner<void>('Cleaning', async () => await execa('rimraf', [`${process.cwd()}/dist`]));

export const prepare = useSpinner<void>('Preparing', async () => {
  // Make sure a local tsconfig exists.  Otherwise this will work, but have odd behavior
  let filePath = path.resolve(process.cwd(), 'tsconfig.json');
  if (!fs.existsSync(filePath)) {
    const srcFile = path.resolve(__dirname, '../../config/tsconfig.plugin.local.json');
    fs.copyFile(srcFile, filePath, err => {
      if (err) {
        throw err;
      }
      console.log(`Created: ${filePath}`);
    });
  }
  // Make sure a local .prettierrc.js exists.  Otherwise this will work, but have odd behavior
  filePath = path.resolve(process.cwd(), '.prettierrc.js');
  if (!fs.existsSync(filePath)) {
    const srcFile = path.resolve(__dirname, '../../config/prettier.plugin.rc.js');
    fs.copyFile(srcFile, filePath, err => {
      if (err) {
        throw err;
      }
      console.log(`Created: ${filePath}`);
    });
  }
  return Promise.resolve();
});

// @ts-ignore
const typecheckPlugin = useSpinner<void>('Typechecking', async () => {
  await execa('tsc', ['--noEmit']);
});

const getTypescriptSources = () => {
  const globPattern = path.resolve(process.cwd(), 'src/**/*.+(ts|tsx)');
  return glob.sync(globPattern);
};

const getStylesSources = () => {
  const globPattern = path.resolve(process.cwd(), 'src/**/*.+(scss|css)');
  return glob.sync(globPattern);
};

export const prettierCheckPlugin = useSpinner<Fixable>('Prettier check', async ({ fix }) => {
  const prettierConfig = require(path.resolve(__dirname, '../../config/prettier.plugin.config.json'));
  const sources = [...getStylesSources(), ...getTypescriptSources()];

  const promises = sources.map((s, i) => {
    return new Promise<{ path: string; failed: boolean }>((resolve, reject) => {
      fs.readFile(s, (err, data) => {
        let failed = false;
        if (err) {
          throw new Error(err.message);
        }

        const opts = {
          ...prettierConfig,
          filepath: s,
        };
        if (!prettier.check(data.toString(), opts)) {
          if (fix) {
            const fixed = prettier.format(data.toString(), opts);
            if (fixed && fixed.length > 10) {
              fs.writeFile(s, fixed, err => {
                if (err) {
                  console.log('Error fixing ' + s, err);
                  failed = true;
                } else {
                  console.log('Fixed: ' + s);
                }
              });
            } else {
              console.log('No automatic fix for: ' + s);
              failed = true;
            }
          } else {
            failed = true;
          }
        }

        resolve({
          path: s,
          failed,
        });
      });
    });
  });

  const results = await Promise.all(promises);
  const failures = results.filter(r => r.failed);
  if (failures.length) {
    console.log('\nFix Prettier issues in following files:');
    failures.forEach(f => console.log(f.path));
    console.log('\nRun toolkit:dev to fix errors');
    throw new Error('Prettier failed');
  }
});

// @ts-ignore
export const lintPlugin = useSpinner<Fixable>('Linting', async ({ fix }) => {
  let tsLintConfigPath = path.resolve(process.cwd(), 'tslint.json');
  if (!fs.existsSync(tsLintConfigPath)) {
    tsLintConfigPath = path.resolve(__dirname, '../../config/tslint.plugin.json');
  }
  const options = {
    fix: fix === true,
    formatter: 'json',
  };

  const configuration = Configuration.findConfiguration(tsLintConfigPath).results;
  const sourcesToLint = getTypescriptSources();

  const lintResults = sourcesToLint
    .map(fileName => {
      const linter = new Linter(options);
      const fileContents = fs.readFileSync(fileName, 'utf8');
      linter.lint(fileName, fileContents, configuration);
      return linter.getResult();
    })
    .filter(result => {
      return result.errorCount > 0 || result.warningCount > 0;
    });

  if (lintResults.length > 0) {
    console.log('\n');
    const failures = lintResults.reduce<RuleFailure[]>((failures, result) => {
      return [...failures, ...result.failures];
    }, []);
    failures.forEach(f => {
      // tslint:disable-next-line
      console.log(
        `${f.getRuleSeverity() === 'warning' ? 'WARNING' : 'ERROR'}: ${
          f.getFileName().split('src')[1]
        }[${f.getStartPosition().getLineAndCharacter().line + 1}:${
          f.getStartPosition().getLineAndCharacter().character
        }]: ${f.getFailure()}`
      );
    });
    console.log('\n');
    throw new Error(`${failures.length} linting errors found in ${lintResults.length} files`);
  }
});

export const pluginBuildRunner: TaskRunner<PluginBuildOptions> = async ({ coverage }) => {
  await clean();
  await prepare();
  await prettierCheckPlugin({ fix: false });
  await lintPlugin({ fix: false });
  await testPlugin({ updateSnapshot: false, coverage, watch: false });
  await bundlePlugin({ watch: false, production: true });
};

export const pluginBuildTask = new Task<PluginBuildOptions>('Build plugin', pluginBuildRunner);
