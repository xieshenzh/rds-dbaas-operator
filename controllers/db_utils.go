/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	cryptorand "crypto/rand"
	"math/big"
	"math/rand"

	"k8s.io/utils/pointer"
)

const (
	postgres           = "postgres"
	auroraPostgresql   = "aurora-postgresql"
	mysql              = "mysql"
	mariadb            = "mariadb"
	aurora             = "aurora"
	auroraMysql        = "aurora-mysql"
	oracleSe2          = "oracle-se2"
	oracleSe2Cdb       = "oracle-se2-cdb"
	oracleEe           = "oracle-ee"
	oracleEeCdb        = "oracle-ee-cdb"
	customOracleEe     = "custom-oracle-ee"
	sqlserverEe        = "sqlserver-ee"
	sqlserverSe        = "sqlserver-se"
	sqlserverEx        = "sqlserver-ex"
	sqlserverWeb       = "sqlserver-web"
	customSqlserverEe  = "custom-sqlserver-ee"
	customSqlserverSe  = "custom-sqlserver-se"
	customSqlserverWeb = "custom-sqlserver-web"

	postgresEngineVersion  = "13.7"
	sqlserverEngineVersion = "15.00.4236.7.v1"
	oracleEngineVersion    = "19.0.0.0.ru-2022-10.rur-2022-10.r1"
	mysqlEngineVersion     = "8.0.28"
	mariadbEngineVersion   = "10.6.10"

	digits   = "0123456789"
	specials = "~=+%^*()[]{}!#$?|"
	letter   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	all      = letter + digits + specials
)

var availabilityZones = map[string][]string{
	"us-east-2": {
		"us-east-2a",
		"us-east-2b",
		"us-east-2c",
	},
	"us-east-1": {
		"us-east-1a",
		"us-east-1b",
		"us-east-1c",
		"us-east-1d",
		"us-east-1e",
		"us-east-1f",
	},
	"us-west-1": {
		"us-west-1a",
		"us-west-1b",
		"us-west-1c",
	},
	"us-west-2": {
		"us-west-2a",
		"us-west-2b",
		"us-west-2c",
		"us-west-2d",
	},
	"af-south-1": {
		"af-south-1a",
		"af-south-1b",
		"af-south-1c",
	},
	"ap-east-1": {
		"ap-east-1a",
		"ap-east-1b",
		"ap-east-1c",
	},
	"ap-south-2": {
		"ap-south-2a",
		"ap-south-2b",
		"ap-south-2c",
	},
	"ap-southeast-3": {
		"ap-southeast-3a",
		"ap-southeast-3b",
		"ap-southeast-3c",
	},
	"ap-south-1": {
		"ap-south-1a",
		"ap-south-1b",
		"ap-south-1c",
	},
	"ap-northeast-3": {
		"ap-northeast-3a",
		"ap-northeast-3b",
		"ap-northeast-3c",
	},
	"ap-northeast-2": {
		"ap-northeast-2a",
		"ap-northeast-2b",
		"ap-northeast-2c",
		"ap-northeast-2d",
	},
	"ap-southeast-1": {
		"ap-southeast-1a",
		"ap-southeast-1b",
		"ap-southeast-1c",
	},
	"ap-southeast-2": {
		"ap-southeast-2a",
		"ap-southeast-2b",
		"ap-southeast-2c",
	},
	"ap-northeast-1": {
		"ap-northeast-1a",
		"ap-northeast-1b",
		"ap-northeast-1c",
		"ap-northeast-1d",
	},
	"ca-central-1": {
		"ca-central-1a",
		"ca-central-1b",
		"ca-central-1c",
		"ca-central-1d",
	},
	"eu-central-1": {
		"eu-central-1a",
		"eu-central-1b",
		"eu-central-1c",
	},
	"eu-west-1": {
		"eu-west-1a",
		"eu-west-1b",
		"eu-west-1c",
	},
	"eu-west-2": {
		"eu-west-2a",
		"eu-west-2b",
		"eu-west-2c",
	},
	"eu-south-1": {
		"eu-south-1a",
		"eu-south-1b",
		"eu-south-1c",
	},
	"eu-west-3": {
		"eu-west-3a",
		"eu-west-3b",
		"eu-west-3c",
	},
	"eu-south-2": {
		"eu-south-2a",
		"eu-south-2b",
		"eu-south-2c",
	},
	"eu-north-1": {
		"eu-north-1a",
		"eu-north-1b",
		"eu-north-1c",
	},
	"eu-central-2": {
		"eu-central-2a",
		"eu-central-2b",
		"eu-central-2c",
	},
	"me-south-1": {
		"me-south-1a",
		"me-south-1b",
		"me-south-1c",
	},
	"me-central-1": {
		"me-central-1a",
		"me-central-1b",
		"me-central-1c",
	},
	"sa-east-1": {
		"sa-east-1a",
		"sa-east-1b",
		"sa-east-1c",
	},
	"us-gov-east-1": {
		"us-gov-east-1a",
		"us-gov-east-1b",
		"us-gov-east-1c",
	},
	"us-gov-west-1": {
		"us-gov-west-1a",
		"us-gov-west-1b",
		"us-gov-west-1c",
	},
}

func generateUsername(engine string) string {
	if engine == "postgres" || engine == "aurora-postgresql" {
		return "postgres"
	} else {
		return "admin"
	}
}

func generateDBName(engine string) *string {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.String("postgres")
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.String("mydb")
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.String("ORCL")
	default:
		return nil
	}
}

func generateBindingType(engine string) string {
	switch engine {
	case postgres, auroraPostgresql:
		return "postgresql"
	case mysql, mariadb, aurora, auroraMysql:
		return "mysql"
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return "oracle"
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return "sqlserver"
	default:
		return ""
	}
}

func getDefaultDBName(engine string) *string {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.String("postgres")
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return pointer.String("master")
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.String("ORCL")
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.String("mysql")
	default:
		return nil
	}
}

func getDefaultEngineVersion(engine *string) *string {
	if engine == nil {
		return nil
	}

	switch *engine {
	case postgres:
		return pointer.String(postgresEngineVersion)
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb:
		return pointer.String(sqlserverEngineVersion)
	case oracleSe2, oracleSe2Cdb:
		return pointer.String(oracleEngineVersion)
	case mysql:
		return pointer.String(mysqlEngineVersion)
	case mariadb:
		return pointer.String(mariadbEngineVersion)
	default:
		return nil
	}
}

func getDefaultAvailabilityZone(region string) *string {
	availabilityZone := region + "a"
	if azs, ok := availabilityZones[region]; ok {
		for _, az := range azs {
			if az == availabilityZone {
				return pointer.String(az)
			}
		}
	}
	return nil
}

func getDefaultDBPort(engine string) *int64 {
	switch engine {
	case postgres, auroraPostgresql:
		return pointer.Int64(5432)
	case sqlserverEe, sqlserverSe, sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
		return pointer.Int64(1433)
	case oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe:
		return pointer.Int64(1521)
	case mysql, mariadb, aurora, auroraMysql:
		return pointer.Int64(3306)
	default:
		return nil
	}
}

func getDBEngineAbbreviation(engine *string) string {
	if engine == nil {
		return ""
	}

	switch *engine {
	case mysql:
		return "mysql-"
	case mariadb:
		return "mariadb-"
	case aurora:
		return "aurora-"
	case auroraMysql:
		return "aurora-mysql-"
	case postgres:
		return "postgres-"
	case auroraPostgresql:
		return "aurora-postgres-"
	case sqlserverEe:
		return "mssql-ee-"
	case sqlserverSe:
		return "mssql-se-"
	case sqlserverEx:
		return "mssql-ex-"
	case sqlserverWeb:
		return "mssql-web-"
	case customSqlserverEe:
		return "cust-mssql-ee-"
	case customSqlserverSe:
		return "cust-mssql-se-"
	case customSqlserverWeb:
		return "cust-mssql-web-"
	case oracleSe2:
		return "oracle-se2-"
	case oracleSe2Cdb:
		return "oracle-se2-cdb-"
	case oracleEe:
		return "oracle-ee-"
	case oracleEeCdb:
		return "oracle-ee-cdb-"
	case customOracleEe:
		return "cust-oracle-ee-"
	default:
		return ""
	}
}

func generatePassword() string {
	length := 12
	buf := make([]byte, length)
	buf[0] = digits[getRandInt(len(digits))]
	buf[1] = specials[getRandInt(len(specials))]
	buf[2] = letter[getRandInt(len(letter))]
	for i := 3; i < length; i++ {
		buf[i] = all[getRandInt(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})
	return string(buf) // E.g. "3i[g0|)z"
}

func getRandInt(s int) int64 {
	result, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(s)))
	return result.Int64()
}
