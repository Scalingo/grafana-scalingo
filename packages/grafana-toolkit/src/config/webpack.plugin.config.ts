const fs = require('fs');
const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const ReplaceInFileWebpackPlugin = require('replace-in-file-webpack-plugin');
const TerserPlugin = require('terser-webpack-plugin');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const OptimizeCssAssetsPlugin = require('optimize-css-assets-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

import * as webpack from 'webpack';
import { getStyleLoaders, getStylesheetEntries, getFileLoaders } from './webpack/loaders';

interface WebpackConfigurationOptions {
  watch?: boolean;
  production?: boolean;
}
type WebpackConfigurationGetter = (options: WebpackConfigurationOptions) => webpack.Configuration;

export const findModuleFiles = (base: string, files?: string[], result?: string[]) => {
  files = files || fs.readdirSync(base);
  result = result || [];

  if (files) {
    files.forEach(file => {
      const newbase = path.join(base, file);
      if (fs.statSync(newbase).isDirectory()) {
        result = findModuleFiles(newbase, fs.readdirSync(newbase), result);
      } else {
        const filename = path.basename(file);
        if (/^module.(t|j)sx?$/.exec(filename)) {
          // @ts-ignore
          result.push(newbase);
        }
      }
    });
  }
  return result;
};

const getModuleFiles = () => {
  return findModuleFiles(path.resolve(process.cwd(), 'src'));
};

const getManualChunk = (id: string) => {
  if (id.endsWith('module.ts') || id.endsWith('module.js') || id.endsWith('module.tsx')) {
    const idx = id.lastIndexOf(path.sep + 'src' + path.sep);
    if (idx > 0) {
      const name = id.substring(idx + 5, id.lastIndexOf('.'));

      return {
        name,
        module: id,
      };
    }
  }
  return null;
};

const getEntries = () => {
  const entries: { [key: string]: string } = {};
  const modules = getModuleFiles();

  modules.forEach(modFile => {
    const mod = getManualChunk(modFile);
    // @ts-ignore
    entries[mod.name] = mod.module;
  });
  return {
    ...entries,
    ...getStylesheetEntries(),
  };
};

const getCommonPlugins = (options: WebpackConfigurationOptions) => {
  const packageJson = require(path.resolve(process.cwd(), 'package.json'));
  return [
    new MiniCssExtractPlugin({
      // both options are optional
      filename: 'styles/[name].css',
    }),
    new webpack.optimize.OccurrenceOrderPlugin(true),
    new CopyWebpackPlugin(
      [
        { from: 'plugin.json', to: '.' },
        { from: '../README.md', to: '.' },
        { from: '../LICENSE', to: '.' },
        { from: '**/*.json', to: '.' },
        { from: '**/*.svg', to: '.' },
        { from: '**/*.png', to: '.' },
        { from: '**/*.html', to: '.' },
        { from: 'img/**/*', to: '.' },
        { from: 'libs/**/*', to: '.' },
        { from: 'static/**/*', to: '.' },
      ],
      { logLevel: options.watch ? 'silent' : 'warn' }
    ),

    new ReplaceInFileWebpackPlugin([
      {
        dir: 'dist',
        files: ['plugin.json', 'README.md'],
        rules: [
          {
            search: '%VERSION%',
            replace: packageJson.version,
          },
          {
            search: '%TODAY%',
            replace: new Date().toISOString().substring(0, 10),
          },
        ],
      },
    ]),
  ];
};

export const getWebpackConfig: WebpackConfigurationGetter = options => {
  const plugins = getCommonPlugins(options);
  const optimization: { [key: string]: any } = {};

  if (options.production) {
    optimization.minimizer = [new TerserPlugin(), new OptimizeCssAssetsPlugin()];
  } else if (options.watch) {
    plugins.push(new HtmlWebpackPlugin());
  }

  return {
    mode: options.production ? 'production' : 'development',
    target: 'web',
    node: {
      fs: 'empty',
      net: 'empty',
      tls: 'empty',
    },
    context: path.join(process.cwd(), 'src'),
    devtool: 'source-map',
    entry: getEntries(),
    output: {
      filename: '[name].js',
      path: path.join(process.cwd(), 'dist'),
      libraryTarget: 'amd',
      publicPath: '/',
    },

    performance: { hints: false },
    externals: [
      'lodash',
      'jquery',
      'moment',
      'slate',
      'emotion',
      'prismjs',
      'slate-plain-serializer',
      '@grafana/slate-react',
      'react',
      'react-dom',
      'react-redux',
      'redux',
      'rxjs',
      'd3',
      'angular',
      '@grafana/ui',
      '@grafana/runtime',
      '@grafana/data',
      // @ts-ignore
      (context, request, callback) => {
        const prefix = 'grafana/';
        if (request.indexOf(prefix) === 0) {
          return callback(null, request.substr(prefix.length));
        }

        // @ts-ignore
        callback();
      },
    ],
    plugins,
    resolve: {
      extensions: ['.ts', '.tsx', '.js'],
      modules: [path.resolve(process.cwd(), 'src'), 'node_modules'],
    },
    module: {
      rules: [
        {
          test: /\.tsx?$/,
          loaders: [
            {
              loader: 'babel-loader',
              options: {
                presets: [['@babel/preset-env', { modules: false }]],
                plugins: ['angularjs-annotate'],
              },
            },
            {
              loader: 'ts-loader',
              options: { onlyCompileBundledFiles: true },
            },
          ],
          exclude: /(node_modules)/,
        },
        {
          test: /\.jsx?$/,
          loaders: [
            {
              loader: 'babel-loader',
              options: {
                presets: [['@babel/preset-env', { modules: false }]],
                plugins: ['angularjs-annotate'],
              },
            },
          ],
          exclude: /(node_modules)/,
        },
        ...getStyleLoaders(),
        {
          test: /\.html$/,
          exclude: [/node_modules/],
          use: {
            loader: 'html-loader',
          },
        },
        ...getFileLoaders(),
      ],
    },
    optimization,
  };
};
