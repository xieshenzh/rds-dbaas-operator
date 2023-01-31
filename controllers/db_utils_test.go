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
	"math/rand"
	"unicode"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"
)

var _ = Describe("DBUtils", func() {
	Context("Generate Username", func() {
		DescribeTable("checking generateUsername",
			func(engine string, username string) {
				u := generateUsername(engine)
				Expect(u).Should(Equal(username))
			},

			Entry("aurora", "aurora", "admin"),
			Entry("aurora-mysql", "aurora-mysql", "admin"),
			Entry("aurora-postgresql", "aurora-postgresql", "postgres"),
			Entry("custom-oracle-ee", "custom-oracle-ee", "admin"),
			Entry("mariadb", "mariadb", "admin"),
			Entry("mysql", "mysql", "admin"),
			Entry("oracle-ee", "oracle-ee", "admin"),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", "admin"),
			Entry("oracle-se2", "oracle-se2", "admin"),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", "admin"),
			Entry("postgres", "postgres", "postgres"),
			Entry("sqlserver-ee", "sqlserver-ee", "admin"),
			Entry("sqlserver-se", "sqlserver-se", "admin"),
			Entry("sqlserver-ex", "sqlserver-ex", "admin"),
			Entry("sqlserver-web", "sqlserver-web", "admin"),
			Entry("custom-sqlserver-ee", "custom-sqlserver-ee", "admin"),
			Entry("custom-sqlserver-se", "custom-sqlserver-se", "admin"),
			Entry("custom-sqlserver-web", "custom-sqlserver-web", "admin"),
		)
	})

	Context("Generate DB Name", func() {
		DescribeTable("checking generateDBName",
			func(engine string, dbName *string) {
				n := generateDBName(engine)
				if dbName == nil {
					Expect(n).Should(BeNil())
				} else {
					Expect(*n).Should(Equal(*dbName))
				}
			},

			Entry("aurora", "aurora", pointer.String("mydb")),
			Entry("aurora-mysql", "aurora-mysql", pointer.String("mydb")),
			Entry("aurora-postgresql", "aurora-postgresql", pointer.String("postgres")),
			Entry("custom-oracle-ee", "custom-oracle-ee", pointer.String("ORCL")),
			Entry("mariadb", "mariadb", pointer.String("mydb")),
			Entry("mysql", "mysql", pointer.String("mydb")),
			Entry("oracle-ee", "oracle-ee", pointer.String("ORCL")),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", pointer.String("ORCL")),
			Entry("oracle-se2", "oracle-se2", pointer.String("ORCL")),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", pointer.String("ORCL")),
			Entry("postgres", "postgres", pointer.String("postgres")),
			Entry("sqlserver-ee", "sqlserver-ee", nil),
			Entry("sqlserver-se", "sqlserver-se", nil),
			Entry("sqlserver-ex", "sqlserver-ex", nil),
			Entry("sqlserver-web", "sqlserver-web", nil),
			Entry("custom-sqlserver-ee", "custom-sqlserver-ee", nil),
			Entry("custom-sqlserver-se", "custom-sqlserver-se", nil),
			Entry("custom-sqlserver-web", "custom-sqlserver-web", nil),
			Entry("Invalid", "invalid", nil),
		)
	})

	Context("Generate Binding Type", func() {
		DescribeTable("checking generateBindingType",
			func(engine string, bt string) {
				t := generateBindingType(engine)
				Expect(t).Should(Equal(bt))
			},

			Entry("aurora", "aurora", "mysql"),
			Entry("aurora-mysql", "aurora-mysql", "mysql"),
			Entry("aurora-postgresql", "aurora-postgresql", "postgresql"),
			Entry("custom-oracle-ee", "custom-oracle-ee", "oracle"),
			Entry("mariadb", "mariadb", "mysql"),
			Entry("mysql", "mysql", "mysql"),
			Entry("oracle-ee", "oracle-ee", "oracle"),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", "oracle"),
			Entry("oracle-se2", "oracle-se2", "oracle"),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", "oracle"),
			Entry("postgres", "postgres", "postgresql"),
			Entry("sqlserver-ee", "sqlserver-ee", "sqlserver"),
			Entry("sqlserver-se", "sqlserver-se", "sqlserver"),
			Entry("sqlserver-ex", "sqlserver-ex", "sqlserver"),
			Entry("sqlserver-web", "sqlserver-web", "sqlserver"),
			Entry("custom-sqlserver-ee", "custom-sqlserver-ee", "sqlserver"),
			Entry("custom-sqlserver-se", "custom-sqlserver-se", "sqlserver"),
			Entry("custom-sqlserver-web", "custom-sqlserver-web", "sqlserver"),
		)
	})

	Context("Get Default DB Name", func() {
		DescribeTable("checking getDefaultDBName",
			func(engine string, dbName *string) {
				n := getDefaultDBName(engine)
				if dbName == nil {
					Expect(n).Should(BeNil())
				} else {
					Expect(*n).Should(Equal(*dbName))
				}
			},

			Entry("aurora", "aurora", pointer.String("mysql")),
			Entry("aurora-mysql", "aurora-mysql", pointer.String("mysql")),
			Entry("aurora-postgresql", "aurora-postgresql", pointer.String("postgres")),
			Entry("custom-oracle-ee", "custom-oracle-ee", pointer.String("ORCL")),
			Entry("mariadb", "mariadb", pointer.String("mysql")),
			Entry("mysql", "mysql", pointer.String("mysql")),
			Entry("oracle-ee", "oracle-ee", pointer.String("ORCL")),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", pointer.String("ORCL")),
			Entry("oracle-se2", "oracle-se2", pointer.String("ORCL")),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", pointer.String("ORCL")),
			Entry("postgres", "postgres", pointer.String("postgres")),
			Entry("sqlserver-ee", "sqlserver-ee", pointer.String("master")),
			Entry("sqlserver-se", "sqlserver-se", pointer.String("master")),
			Entry("sqlserver-ex", "sqlserver-ex", pointer.String("master")),
			Entry("sqlserver-web", "sqlserver-web", pointer.String("master")),
			Entry("custom-sqlserver-ee", "custom-sqlserver-ee", pointer.String("master")),
			Entry("custom-sqlserver-se", "custom-sqlserver-se", pointer.String("master")),
			Entry("custom-sqlserver-web", "custom-sqlserver-web", pointer.String("master")),
			Entry("Invalid", "invalid", nil),
		)
	})

	Context("Generate Default DB Port", func() {
		DescribeTable("checking getDefaultDBPort",
			func(engine string, dbPort *int64) {
				p := getDefaultDBPort(engine)
				if dbPort == nil {
					Expect(p).Should(BeNil())
				} else {
					Expect(*p).Should(Equal(*dbPort))
				}
			},

			Entry("aurora", "aurora", pointer.Int64(3306)),
			Entry("aurora-mysql", "aurora-mysql", pointer.Int64(3306)),
			Entry("aurora-postgresql", "aurora-postgresql", pointer.Int64(5432)),
			Entry("custom-oracle-ee", "custom-oracle-ee", pointer.Int64(1521)),
			Entry("mariadb", "mariadb", pointer.Int64(3306)),
			Entry("mysql", "mysql", pointer.Int64(3306)),
			Entry("oracle-ee", "oracle-ee", pointer.Int64(1521)),
			Entry("oracle-ee-cdb", "oracle-ee-cdb", pointer.Int64(1521)),
			Entry("oracle-se2", "oracle-se2", pointer.Int64(1521)),
			Entry("oracle-se2-cdb", "oracle-se2-cdb", pointer.Int64(1521)),
			Entry("postgres", "postgres", pointer.Int64(5432)),
			Entry("sqlserver-ee", "sqlserver-ee", pointer.Int64(1433)),
			Entry("sqlserver-se", "sqlserver-se", pointer.Int64(1433)),
			Entry("sqlserver-ex", "sqlserver-ex", pointer.Int64(1433)),
			Entry("sqlserver-web", "sqlserver-web", pointer.Int64(1433)),
			Entry("custom-sqlserver-ee", "custom-sqlserver-ee", pointer.Int64(1433)),
			Entry("custom-sqlserver-se", "custom-sqlserver-se", pointer.Int64(1433)),
			Entry("custom-sqlserver-web", "custom-sqlserver-web", pointer.Int64(1433)),
			Entry("Invalid", "invalid", nil),
		)
	})

	Context("Get Default Engine Version", func() {
		DescribeTable("checking getDefaultEngineVersion",
			func(engine *string, version *string) {
				v := getDefaultEngineVersion(engine)
				if version == nil {
					Expect(v).Should(BeNil())
				} else {
					Expect(*v).Should(Equal(*version))
				}
			},

			Entry("aurora", pointer.String("aurora"), nil),
			Entry("aurora-mysql", pointer.String("aurora-mysql"), nil),
			Entry("aurora-postgresql", pointer.String("aurora-postgresql"), nil),
			Entry("custom-oracle-ee", pointer.String("custom-oracle-ee"), nil),
			Entry("mariadb", pointer.String("mariadb"), pointer.String("10.6.10")),
			Entry("mysql", pointer.String("mysql"), pointer.String("8.0.28")),
			Entry("oracle-ee", pointer.String("oracle-ee"), nil),
			Entry("oracle-ee-cdb", pointer.String("oracle-ee-cdb"), nil),
			Entry("oracle-se2", pointer.String("oracle-se2"), pointer.String("19.0.0.0.ru-2022-10.rur-2022-10.r1")),
			Entry("oracle-se2-cdb", pointer.String("oracle-se2-cdb"), pointer.String("19.0.0.0.ru-2022-10.rur-2022-10.r1")),
			Entry("postgres", pointer.String("postgres"), pointer.String("13.7")),
			Entry("sqlserver-ee", pointer.String("sqlserver-ee"), pointer.String("15.00.4236.7.v1")),
			Entry("sqlserver-se", pointer.String("sqlserver-se"), pointer.String("15.00.4236.7.v1")),
			Entry("sqlserver-ex", pointer.String("sqlserver-ex"), pointer.String("15.00.4236.7.v1")),
			Entry("sqlserver-web", pointer.String("sqlserver-web"), pointer.String("15.00.4236.7.v1")),
			Entry("custom-sqlserver-ee", pointer.String("custom-sqlserver-ee"), nil),
			Entry("custom-sqlserver-se", pointer.String("custom-sqlserver-se"), nil),
			Entry("custom-sqlserver-web", pointer.String("custom-sqlserver-web"), nil),
			Entry("Invalid", pointer.String("invalid"), nil),
			Entry("nil", nil, nil),
		)
	})

	Context("Get Default Availability Zone", func() {
		DescribeTable("checking getDefaultAvailabilityZone",
			func(region string, az *string) {
				v := getDefaultAvailabilityZone(region)
				if az == nil {
					Expect(v).Should(BeNil())
				} else {
					Expect(*v).Should(Equal(*az))
				}
			},

			Entry("us-east-2", "us-east-2", pointer.String("us-east-2a")),
			Entry("us-east-1", "us-east-1", pointer.String("us-east-1a")),
			Entry("us-west-1", "us-west-1", pointer.String("us-west-1a")),
			Entry("us-west-2", "us-west-2", pointer.String("us-west-2a")),
			Entry("af-south-1", "af-south-1", pointer.String("af-south-1a")),
			Entry("ap-east-1", "ap-east-1", pointer.String("ap-east-1a")),
			Entry("ap-south-2", "ap-south-2", pointer.String("ap-south-2a")),
			Entry("ap-southeast-3", "ap-southeast-3", pointer.String("ap-southeast-3a")),
			Entry("ap-south-1", "ap-south-1", pointer.String("ap-south-1a")),
			Entry("ap-northeast-3", "ap-northeast-3", pointer.String("ap-northeast-3a")),
			Entry("ap-northeast-2", "ap-northeast-2", pointer.String("ap-northeast-2a")),
			Entry("ap-southeast-1", "ap-southeast-1", pointer.String("ap-southeast-1a")),
			Entry("ap-southeast-2", "ap-southeast-2", pointer.String("ap-southeast-2a")),
			Entry("ap-northeast-1", "ap-northeast-1", pointer.String("ap-northeast-1a")),
			Entry("ca-central-1", "ca-central-1", pointer.String("ca-central-1a")),
			Entry("eu-central-1", "eu-central-1", pointer.String("eu-central-1a")),
			Entry("eu-west-1", "eu-west-1", pointer.String("eu-west-1a")),
			Entry("eu-west-2", "eu-west-2", pointer.String("eu-west-2a")),
			Entry("eu-south-1", "eu-south-1", pointer.String("eu-south-1a")),
			Entry("eu-west-3", "eu-west-3", pointer.String("eu-west-3a")),
			Entry("eu-south-2", "eu-south-2", pointer.String("eu-south-2a")),
			Entry("eu-north-1", "eu-north-1", pointer.String("eu-north-1a")),
			Entry("eu-central-2", "eu-central-2", pointer.String("eu-central-2a")),
			Entry("me-south-1", "me-south-1", pointer.String("me-south-1a")),
			Entry("me-central-1", "me-central-1", pointer.String("me-central-1a")),
			Entry("sa-east-1", "sa-east-1", pointer.String("sa-east-1a")),
			Entry("us-gov-east-1", "us-gov-east-1", pointer.String("us-gov-east-1a")),
			Entry("us-gov-west-1", "us-gov-west-1", pointer.String("us-gov-west-1a")),
			Entry("Invalid", "invalid", nil),
			Entry("nil", "", nil),
		)
	})

	Context("Generate DB Engine Abbreviation", func() {
		DescribeTable("checking getDBEngineAbbreviation",
			func(engine *string, abbr string) {
				n := getDBEngineAbbreviation(engine)
				Expect(n).Should(Equal(abbr))
			},

			Entry("aurora", pointer.String("aurora"), "aurora-"),
			Entry("aurora-mysql", pointer.String("aurora-mysql"), "aurora-mysql-"),
			Entry("aurora-postgresql", pointer.String("aurora-postgresql"), "aurora-postgres-"),
			Entry("custom-oracle-ee", pointer.String("custom-oracle-ee"), "cust-oracle-ee-"),
			Entry("mariadb", pointer.String("mariadb"), "mariadb-"),
			Entry("mysql", pointer.String("mysql"), "mysql-"),
			Entry("oracle-ee", pointer.String("oracle-ee"), "oracle-ee-"),
			Entry("oracle-ee-cdb", pointer.String("oracle-ee-cdb"), "oracle-ee-cdb-"),
			Entry("oracle-se2", pointer.String("oracle-se2"), "oracle-se2-"),
			Entry("oracle-se2-cdb", pointer.String("oracle-se2-cdb"), "oracle-se2-cdb-"),
			Entry("postgres", pointer.String("postgres"), "postgres-"),
			Entry("sqlserver-ee", pointer.String("sqlserver-ee"), "mssql-ee-"),
			Entry("sqlserver-se", pointer.String("sqlserver-se"), "mssql-se-"),
			Entry("sqlserver-ex", pointer.String("sqlserver-ex"), "mssql-ex-"),
			Entry("sqlserver-web", pointer.String("sqlserver-web"), "mssql-web-"),
			Entry("custom-sqlserver-ee", pointer.String("custom-sqlserver-ee"), "cust-mssql-ee-"),
			Entry("custom-sqlserver-se", pointer.String("custom-sqlserver-se"), "cust-mssql-se-"),
			Entry("custom-sqlserver-web", pointer.String("custom-sqlserver-web"), "cust-mssql-web-"),
			Entry("Invalid", pointer.String("invalid"), ""),
			Entry("Blank", pointer.String(""), ""),
			Entry("Nil", nil, ""),
		)
	})

	Context("Generate Password", func() {
		DescribeTable("checking generatePassword",
			func() {
				s := generatePassword()
				Expect(len(s)).Should(BeNumerically("==", 12))
				special := false
				number := false
				letter := false
				unexpected := false
				for _, c := range s {
					switch {
					case unicode.IsNumber(c):
						number = true
					case unicode.IsPunct(c) || unicode.IsSymbol(c):
						special = true
					case unicode.IsLetter(c):
						letter = true
					default:
						unexpected = true
					}
				}
				Expect(special).Should(BeTrue())
				Expect(number).Should(BeTrue())
				Expect(letter).Should(BeTrue())
				Expect(unexpected).Should(BeFalse())
			},
			Entry("generating password 1"),
			Entry("generating password 2"),
			Entry("generating password 3"),
			Entry("generating password 4"),
			Entry("generating password 5"),
			Entry("generating password 6"),
			Entry("generating password 7"),
			Entry("generating password 8"),
			Entry("generating password 9"),
			Entry("generating password 10"),
		)
	})

	Context("Generate Random Int", func() {
		rand.Seed(GinkgoRandomSeed())

		DescribeTable("checking getRandInt",
			func(s int) {
				i := getRandInt(s)
				Expect(i).Should(BeNumerically(">=", 0))
				Expect(i).Should(BeNumerically("<", s))
			},
			Entry("random number 1", rand.Intn(10000)),
			Entry("random number 2", rand.Intn(10000)),
			Entry("random number 3", rand.Intn(10000)),
			Entry("random number 4", rand.Intn(10000)),
			Entry("random number 5", rand.Intn(10000)),
			Entry("random number 6", rand.Intn(10000)),
			Entry("random number 7", rand.Intn(10000)),
			Entry("random number 8", rand.Intn(10000)),
			Entry("random number 9", rand.Intn(10000)),
			Entry("random number 10", rand.Intn(10000)),
		)
	})
})
