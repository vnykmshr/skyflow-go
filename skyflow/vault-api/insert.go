package vaultapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/skyflowapi/skyflow-go/commonutils/errors"
	logger "github.com/skyflowapi/skyflow-go/commonutils/logwrapper"
	"github.com/skyflowapi/skyflow-go/commonutils/messages"
	"github.com/skyflowapi/skyflow-go/skyflow/common"
)

type InsertApi struct {
	Configuration common.Configuration
	Records       map[string]interface{}
	Options       common.InsertOptions
}

var insertTag = "Insert"

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	Client HTTPClient
)

func init() {
	Client = &http.Client{}
}

func (insertApi *InsertApi) doValidations() *errors.SkyflowError {

	var err = isValidVaultDetails(insertApi.Configuration)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf(messages.VALIDATE_RECORDS, insertTag))

	var totalRecords = insertApi.Records["records"]
	if totalRecords == nil {
		logger.Error(fmt.Sprintf(messages.RECORDS_KEY_NOT_FOUND, insertTag))
		return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.RECORDS_KEY_NOT_FOUND, insertTag))
	}
	var recordsArray = (totalRecords).([]interface{})
	if len(recordsArray) == 0 {
		logger.Error(fmt.Sprintf(messages.EMPTY_RECORDS, insertTag))
		return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.EMPTY_RECORDS, insertTag))
	}
	for _, record := range recordsArray {
		var singleRecord = (record).(map[string]interface{})
		var table = singleRecord["table"]
		var fields = singleRecord["fields"]
		if table == nil {
			logger.Error(fmt.Sprintf(messages.MISSING_TABLE, insertTag))
			return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.MISSING_TABLE, insertTag))
		} else if table == "" {
			logger.Error(fmt.Sprintf(messages.EMPTY_TABLE_NAME, insertTag))
			return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.EMPTY_TABLE_NAME, insertTag))
		} else if fields == nil {
			logger.Error(fmt.Sprintf(messages.FIELDS_KEY_ERROR, insertTag))
			return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.FIELDS_KEY_ERROR, insertTag))
		} else if fields == "" {
			logger.Error(fmt.Sprintf(messages.EMPTY_FIELDS, insertTag))
			return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.EMPTY_FIELDS, insertTag))
		}
		field := (singleRecord["fields"]).(map[string]interface{})
		if len(field) == 0 {
			logger.Error(fmt.Sprintf(messages.EMPTY_FIELDS, insertTag))
			return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.EMPTY_FIELDS, insertTag))
		}
		for index := range field {
			if index == "" {
				logger.Error(fmt.Sprintf(messages.EMPTY_COLUMN_NAME, insertTag))
				return errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.EMPTY_COLUMN_NAME, insertTag))
			}
		}
	}
	return nil
}

func (insertApi *InsertApi) Post(token string) (map[string]interface{}, *errors.SkyflowError) {
	err := insertApi.doValidations()
	if err != nil {
		return nil, err
	}
	jsonRecord, _ := json.Marshal(insertApi.Records)
	var insertRecord common.InsertRecord
	if err := json.Unmarshal(jsonRecord, &insertRecord); err != nil {
		logger.Error(fmt.Sprintf(messages.INVALID_RECORDS, insertTag))
		return nil, errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.INVALID_RECORDS, insertTag))
	}

	record, err := insertApi.constructRequestBody(insertRecord, insertApi.Options)
	if err != nil {
		return nil, err
	}
	requestBody, err1 := json.Marshal(record)
	if err1 != nil {
		logger.Error(fmt.Sprintf(messages.EMPTY_RECORDS, detokenizeTag))
		return nil, errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.UNKNOWN_ERROR, insertTag, err1))
	}
	requestUrl := fmt.Sprintf("%s/v1/vaults/%s", insertApi.Configuration.VaultURL, insertApi.Configuration.VaultID)
	request, _ := http.NewRequest(
		"POST",
		requestUrl,
		strings.NewReader(string(requestBody)),
	)
	bearerToken := fmt.Sprintf("Bearer %s", token)
	request.Header.Add("Authorization", bearerToken)

	logger.Info(fmt.Sprintf(messages.INSERTING_RECORDS, insertTag, insertApi.Configuration.VaultID))
	res, err2 := Client.Do(request)
	if err2 != nil {
		logger.Error(fmt.Sprintf(messages.INSERTING_RECORDS_FAILED, insertTag, insertApi.Configuration.VaultID))
		code := strconv.Itoa(res.StatusCode)
		return nil, errors.NewSkyflowError(errors.ErrorCodesEnum(code), fmt.Sprintf(messages.SERVER_ERROR, insertTag, err2))
	}

	logger.Info(fmt.Sprintf(messages.INSERTING_RECORDS_SUCCESS, insertTag, insertApi.Configuration.VaultID))

	data, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	var result map[string]interface{}
	err2 = json.Unmarshal(data, &result)
	if err2 != nil {
		logger.Error(fmt.Sprintf(messages.INSERTING_RECORDS_FAILED, insertTag, insertApi.Configuration.VaultID))
		return nil, errors.NewSkyflowError(errors.ErrorCodesEnum(errors.SdkErrorCode), fmt.Sprintf(messages.UNKNOWN_ERROR, insertTag, string(data)))
	} else if result["error"] != nil {
		logger.Error(fmt.Sprintf(messages.INSERTING_RECORDS_FAILED, insertTag, insertApi.Configuration.VaultID))
		var generatedError = (result["error"]).(map[string]interface{})
		return nil, errors.NewSkyflowError(errors.ErrorCodesEnum(fmt.Sprintf("%v", generatedError["http_code"])), fmt.Sprintf(messages.SERVER_ERROR, insertTag, generatedError["message"]))
	}
	return insertApi.buildResponse((result["responses"]).([]interface{}), insertRecord), nil
}

func (InsertApi *InsertApi) constructRequestBody(record common.InsertRecord, options common.InsertOptions) (map[string]interface{}, *errors.SkyflowError) {
	postPayload := []interface{}{}
	records := record.Records

	for index, value := range records {
		singleRecord := value
		table := singleRecord.Table
		fields := singleRecord.Fields
		var finalRecord = make(map[string]interface{})
		finalRecord["tableName"] = table
		finalRecord["fields"] = fields
		finalRecord["method"] = "POST"
		finalRecord["quorum"] = true
		postPayload = append(postPayload, finalRecord)
		if options.Tokens {
			temp2 := make(map[string]interface{})
			temp2["method"] = "GET"
			temp2["tableName"] = table
			temp2["ID"] = fmt.Sprintf("$responses.%v.records.0.skyflow_id", index)
			temp2["tokenization"] = true
			postPayload = append(postPayload, temp2)
		}

	}
	body := make(map[string]interface{})
	body["records"] = postPayload
	return body, nil
}

func (insertApi *InsertApi) buildResponse(responseJson []interface{}, requestRecords common.InsertRecord) map[string]interface{} {

	var inputRecords = requestRecords.Records
	var recordsArray = []interface{}{}
	var responseObject = make(map[string]interface{})
	if insertApi.Options.Tokens {
		for i := (len(responseJson) / 2); i < len(responseJson); i++ {
			var skyflowIDsObject = (responseJson[i-
				(len(responseJson)-len(responseJson)/2)]).(map[string]interface{})
			var skyflowIDs = (skyflowIDsObject["records"]).([]interface{})
			var skyflowID = (skyflowIDs[0]).(map[string]interface{})["skyflow_id"]
			var record = (responseJson[i]).(map[string]interface{})
			var inputRecord = inputRecords[i-len(responseJson)/2]
			record["table"] = inputRecord.Table
			var fields = (record["fields"]).(map[string]interface{})
			fields["skyflow_id"] = skyflowID
			record["fields"] = fields
			recordsArray = append(recordsArray, record)
		}
	} else {
		for i := 0; i < len(responseJson); i++ {
			var inputRecord = inputRecords[i]
			var record = ((responseJson[i]).(map[string]interface{})["records"]).([]interface{})
			((record[0]).(map[string]interface{}))["table"] = inputRecord.Table
			recordsArray = append(recordsArray, record[0])

		}
	}
	responseObject["records"] = recordsArray
	return responseObject
}
