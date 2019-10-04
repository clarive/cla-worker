import app from '@claw/app';
import * as yargs from 'yargs';
import PubSub from '@claw/pubsub';
import { commonOptions } from '@claw/commands';

module.exports = new class implements yargs.CommandModule {
    command = 'unregister';
    describe = 'Unregister worker from server';

    builder(args: yargs.Argv) {
        commonOptions(args, 'verbose', 'workerid', 'token', 'url', 'origin');
        return args;
    }

    async handler(argv: yargs.Arguments) {
        app.build({ argv });

        try {
            await app.startup();
            const {
                id,
                token,
                url,
                origin,
                passkey,
                tags,
                envs
            } = app.config.data;

            const pubsub = new PubSub({
                id,
                token,
                origin,
                tags,
                envs,
                baseURL: url
            });

            const result = await pubsub.unregister(passkey);
            const { registration, error, projects } = result;

            app.debug('registration', registration);
            app.debug('projects', projects);

            if (error) {
                app.fail(`error unregistering worker: ${error}`);
            } else {
                app.milestone('worker registration removed: ', pubsub.id);
            }
        } catch (err) {
            app.debug(err);
            app.fail('command %s: %s', app.commandName, err);
        }
    }
}();
