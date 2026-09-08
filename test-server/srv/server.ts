import cds from "@sap/cds";
import morgan from "morgan";
import {Application, Request, Response} from "express";
// @ts-ignore
import {addMockService} from "../mock/MockService";

type DumpTellerRow = {
  DATUM: string;
  betaald: number;
  aantal: number;
};

async function fetchDumpTellerRows(): Promise<DumpTellerRow[]> {
  const response = await fetch("http://dummy.fledge.nl/dumpTeller.txt");
  if (!response.ok) {
    throw new Error(`dump teller request failed with ${response.status}`);
  }

  const body = await response.text();
  return body
    .split(/\r?\n/)
    .map((line) => line.trimEnd())
    .filter((line) => /^\d{4}-\d{2}-\d{2}\s+\d+\s+\d+$/.test(line))
    .map((line) => {
      const match = line.match(/^(\d{4}-\d{2}-\d{2})\s+(\d+)\s+(\d+)$/);
      if (!match) {
        throw new Error(`unable to parse dump teller row: ${line}`);
      }
      return {
        DATUM: match[1],
        betaald: Number(match[2]),
        aantal: Number(match[3]),
      };
    });
}

cds.on('bootstrap', async (app: Application) => {
  app.use(morgan('dev'));
  app.get("/dump-teller.json", async (_req: Request, res: Response) => {
    try {
      const rows = await fetchDumpTellerRows();
      res.json(rows);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      res.status(502).json({error: message});
    }
  });
  await addMockService(app);
})
