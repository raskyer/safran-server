package service

import (
	"errors"
	"io"

	"github.com/leaklessgfy/safran-server/entity"
	"github.com/leaklessgfy/safran-server/utils"

	"github.com/leaklessgfy/safran-server/parser"
)

// ImportService is the service use to orchestrate parsing and inserting inside influx
type ImportService struct {
	influx        *InfluxService
	samplesParser *parser.SamplesParser
	alarmsParser  *parser.AlarmsParser
}

// NewImportService create the import service
func NewImportService(influx *InfluxService, samplesReader, alarmsReader io.Reader) (*ImportService, error) {
	err := influx.Ping()

	if err != nil {
		return nil, err
	}

	samplesParser := parser.NewSamplesParser(samplesReader)
	alarmsParser := parser.NewAlarmsParser(alarmsReader)

	return &ImportService{
		influx:        influx,
		samplesParser: samplesParser,
		alarmsParser:  alarmsParser,
	}, nil
}

// ImportExperiment will import the experiment
func (i ImportService) ImportExperiment(experiment entity.Experiment) (entity.Experiment, error) {
	header, err := i.samplesParser.ParseHeader()
	if err != nil {
		return experiment, errors.New("{Parse Header} - " + err.Error())
	}
	experiment.StartDate, err = utils.ParseDate(header.StartDate)
	if err != nil {
		return experiment, errors.New("{Parse Experiment StartDate} - " + err.Error())
	}
	experiment.EndDate, err = utils.ParseDate(header.EndDate)
	if err != nil {
		return experiment, errors.New("{Parse Experiment EndDate} - " + err.Error())
	}
	experiment.ID, err = i.influx.InsertExperiment(experiment)
	if err != nil {
		return experiment, errors.New("{Insert Experiment} - " + err.Error())
	}

	return experiment, nil
}

func (i ImportService) ImportSamples(report entity.Report, experiment entity.Experiment, reports chan entity.Report) {
	measures, err := i.samplesParser.ParseMeasures()
	if err != nil {
		report.AddError(err)
		if errRemove := i.influx.RemoveExperiment(experiment.ID); errRemove != nil {
			report.AddError(errRemove)
		}
		reports <- report
		return
	}

	measuresID, err := i.influx.InsertMeasures(experiment.ID, measures)
	if err != nil {
		report.AddError(err)
		if errRemove := i.influx.RemoveExperiment(experiment.ID); errRemove != nil {
			report.AddError(errRemove)
		}
		reports <- report
		return
	}

	i.samplesParser.ParseSamples(len(measuresID), func(samples []*entity.Sample) {
		err := i.influx.InsertSamples(experiment.ID, measuresID, experiment.StartDate, samples)
		if err != nil {
			//i.influx.RemoveExperiment(experimentID)
			report.AddError(err)
		}
		reports <- report
	})

	reports <- report
}

// ImportAlarms will import the alarms
func (i ImportService) ImportAlarms(report entity.Report, experiment entity.Experiment, reports chan entity.Report) {
	if i.alarmsParser == nil {
		return
	}

	alarms, err := i.alarmsParser.ParseAlarms()
	if err != nil {
		report.AddError(err)
		if errRemove := i.influx.RemoveExperiment(experiment.ID); errRemove != nil {
			report.AddError(errRemove)
		}
		reports <- report
		return
	}

	err = i.influx.InsertAlarms(experiment.ID, experiment.StartDate, alarms)
	if err != nil {
		report.AddError(err)
		if errRemove := i.influx.RemoveExperiment(experiment.ID); errRemove != nil {
			report.AddError(errRemove)
		}
		reports <- report
		return
	}

	reports <- report
}