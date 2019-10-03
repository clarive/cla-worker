export type LogMessage = string | object;

/* Properties<class> ==> excludes functions from a class */
export type Properties<T> = Pick<
    T,
    { [K in keyof T]: T[K] extends Function ? never : K }[keyof T]
>;

/* PartialProperties<class> ==> excludes functions from a class, allowing a partial/optional */
export type PartialProperties<T> = Partial<T>;

export interface GlobalMeta {
    name: string;
    driver: string;
    [key: string]: any;
}

export interface Logger {
    milestone(msg: LogMessage, ...args): void;
    info(msg: LogMessage, ...args);
    warn(msg: LogMessage, ...args);
    debug(msg: LogMessage, ...args);
    error(msg: LogMessage, ...args);
    echo(msg: LogMessage, ...args);
    fatal(code: number, msg: LogMessage, ...args: any[]);
    fatal(msg: LogMessage, ...args: any[]);
}
