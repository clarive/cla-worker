import app from '@claw/app';
import * as yargs from 'yargs';
import PubSub from '@claw/pubsub';
import { actionService } from '@claw/service';

module.exports = new class implements yargs.CommandModule {
    command = 'stop';
    describe = 'stop the Clarive Worker service';

    builder(args: yargs.Argv) {
        return args;
    }

    async handler(argv: yargs.Arguments) {
        app.build({ argv });

        actionService('stop');

        try {
            await app.startup();

            await app.killDaemon(pidfile);
        } catch (err) {
            app.debug(err);
            app.fail('command "stop": %s', err);
        }
    }
}();
