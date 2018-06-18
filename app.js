const execSync = require('child_process').execSync;
const platform = process.platform;
const programPath = process.argv[0]; // 1 for node, 0 for compiled

function exec(command) {

   var result = {};

   try {

      result.stdout = execSync(command).toString();
      result.status = 0;
      result.success = 1;

   } catch( error ) {

      result = error;
      result.success = 0;

   }

   return result;

}

function isRoot() {

   var isRoot = 0;

   if( platform == 'linux' || platform == 'darwin' ) {

      isRoot = process.getuid && process.getuid() === 0;

   } else if( platform == 'win32' ) {

      isRoot = exec('NET SESSION').success;

   }

   return isRoot;
}

if( process.argv[2] == 'run' ){

   var express = require('express');

   var app = express();

   app.get('*', (req, res) => { console.log('Requested url: ' + req.url); res.send('alive!'); });
   app.listen(3838, () => { console.log('server started...'); } );

}

if( isRoot()) {

   var serivceAction = process.argv[2];
   var serviceModule;

   switch( serivceAction ) {
      case 'start':
      case 'stop':
      case 'install':
      case 'uninstall':
         console.log('Service action: ' + serivceAction);
         break;
      default:
         console.error('Service action not supported!');
         return -1;
   }

   switch( platform ) {
      case 'win32':
         serviceModule = 'node-windows';
         break;
      case 'darwin':
         serviceModule = 'node-mac';
         break;
      case 'linux':
         serviceModule = 'node-linux';
         break;
   }

   var Service = require( 'node-linux' ).Service; //serviceModule, Can not be dynamic to compile with pack

   var svc = new Service({
      
      name: 'claw',
      env: [ { name: 'variableName', value: 'value' } ],
      description: 'Clarive worker service',
      script: programPath + ' run',
      user: 'clarive',
      group: 'clarive'
   
   });

   svc.on('start', () => {
      console.log('Clarive Worker service started...');
   } );

   svc.on('stop', () => {
      console.log('Clarive Worker service stoped...');
   } );

   svc.on('alreadyinstalled', () => {
      console.log('Clarive Worker service already installed, no need to do anything...');
   } );

   svc.on('doesnotexist', () => {
      console.log('Clarive Worker service is not installed, please install it first...');
   } ); 
   
   svc.on('uninstall', () => {
      console.log('Clarive Worker service uninstall completed!');
   });

   svc.on('install', () => {
      console.log('Clarive Worker executable path: ' + programPath);
      console.log('Clarive Worker service install completed!');
      svc.start();
   });

   svc[serivceAction]();

} else {

   console.log('Need to be root to manage services!');

}

