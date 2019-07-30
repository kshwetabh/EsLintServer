var fs = require("fs");
const util = require("util");
const CLIEngine = require("eslint").CLIEngine;
const grpc = require("grpc");
var protoLoader = require("@grpc/proto-loader");

const PROTO_PATH = "../proto/eslintmessage.proto";
const PORT = "127.0.0.1:4040";

var packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true
});
var eslintProto = grpc.loadPackageDefinition(packageDefinition).proto;
const cli = initEsLintCLIEngine();

main();

function main() {
  const server = new grpc.Server();
  server.addService(eslintProto.EsLintService.service, { LintFile: LintFile });
  server.bind(PORT, grpc.ServerCredentials.createInsecure());

  console.log("Server running at ", PORT);
  server.start();
}

/**
 *
 * @param {*} fileName
 */
function LintFile(call, callback) {
  //console.log("Got text from client: \n", call.request);
  let data = call.request.fileContent;
  const scanReport = lintFileAsText(data);

  console.log(
    "Sending scanned report to client: \n",
    util.inspect(scanReport, { showHidden: false, depth: null })
  );
  callback(null, {
    errors: scanReport
  });
}

/**
 *
 * @param {*} data File Buffer
 */
function lintFileAsText(data) {
  const report = cli.executeOnText(data.toString(), "tempfile.js"); //FILE_NAME
  const formatter = cli.getFormatter("stylish");
  const formattedReport = formatter(report.results);

  // This is to print complete JSON objects with deep hierarchy
  // console.log(util.inspect(report, { showHidden: false, depth: null }));

  // Return scan report to client
  return formattedReport;
}

function initEsLintCLIEngine() {
  // Initialize ESLint CLIEngine
  // Uses configs from .eslintrc.js file to initialize the engine
  const cli = new CLIEngine({
    envs: ["browser"]
  });

  cli.addPlugin("hms-plugins"); //You can also use "eslint-plugin-hms-plugins"
  return cli;
}
