import app from '@claw/app';
import * as yargs from 'yargs';
import PubSub from '@claw/pubsub';
import { commonOptions, CmdArgs } from '@claw/commands';

module.exports = new class implements yargs.CommandModule {
    command = 'register';
    describe = 'Register worker with a passkey';

    builder(args: yargs.Argv) {
        commonOptions(
            args,
            'verbose',
            'passkey',
            'url',
            'workerid',
            'envs',
            'origin',
            'save',
            'token',
            'config'
        );
        return args;
    }

    async handler(argv: CmdArgs) {
        app.build({ argv });

        try {
            await app.startup();
            const { id, url, origin, passkey, envs, tags } = app.config;
            const { save, config } = argv;

            const pubsub = new PubSub({
                id,
                origin,
                envs,
                tags,
                token: app.config.token,
                baseURL: url
            });

            const result = await pubsub.register(passkey);
            const { token, error, projects } = result;

            if (error) {
                app.fail(`error registering worker: ${error}`);
            } else {
                app.info('Registration WorkerID: ', pubsub.id);
                app.info('Registration token: ', token);
                app.info('Projects registered: ', projects);
                app.info(
                    `Start the worker with the following command:\n\n\tcla-worker run --token ${token} --id ${
                        pubsub.id
                    }\n`
                );
                if (save) {
                    if (
                        config &&
                        typeof config !== 'boolean' &&
                        config.length > 0
                    ) {
                        app.info(
                            `Or with your config file:\n\n\tcla-worker run --id ${
                                pubsub.id
                            } --config ${config}\n`
                        );
                    } else {
                        app.info(
                            `Or from the config file:\n\n\tcla-worker run --id ${
                                pubsub.id
                            }\n`
                        );
                    }
                }

                app.info(`To remove this registration:\n\n\tcla-worker unregister --token ${token} --id ${pubsub.id}\n`);
            }

            if (save) {
                app.info('saving registration to config file...');

                const [configFile] = app.saveConfigFile({
                    registrations: [{ id, token }]
                });

                app.milestone(`registration saved to file '${configFile}'`);
            }
        } catch (err) {
            app.debug(err);
            app.fail('command %s: %s', app.commandName, err);
        }
    }
}();
