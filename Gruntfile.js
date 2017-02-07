/* jshint node:true */
'use strict';
module.exports = function (grunt) {
  var os = require('os');
  var config = {
    pkg: grunt.file.readJSON('package.json'),
    baseDir: '.',
    srcDir: 'public',
    genDir: 'public_gen',
    destDir: 'dist',
    tempDir: 'tmp',
    platform: process.platform.replace('win32', 'windows'),
  };

  if (grunt.option('arch')) {
    config.arch = grunt.option('arch');
  } else {
    config.arch = os.arch();

    if (process.platform.match(/^win/)) {
      config.arch = process.env.hasOwnProperty('ProgramFiles(x86)') ? 'x64' : 'x86';
    }
  }

  config.phjs = grunt.option('phjsToRelease');

  config.pkg.version = grunt.option('pkgVer') || config.pkg.version;
  console.log('Version', config.pkg.version);

  // load plugins
  require('load-grunt-tasks')(grunt);

  // load task definitions
  grunt.loadTasks('tasks');

  // Utility function to load plugin settings into config
  function loadConfig(config,path) {
    require('glob').sync('*', {cwd: path}).forEach(function(option) {
      var key = option.replace(/\.js$/,'');
      // If key already exists, extend it. It is your responsibility to avoid naming collisions
      config[key] = config[key] || {};
      grunt.util._.extend(config[key], require(path + option)(config,grunt));
    });
    // technically not required
    return config;
  }

  // Merge that object with what with whatever we have here
  loadConfig(config,'./tasks/options/');
  // pass the config to grunt
  grunt.initConfig(config);
};
