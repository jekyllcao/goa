package genswagger_test

import (
	"encoding/json"

	"github.com/go-swagger/go-swagger/spec"
	"github.com/go-swagger/go-swagger/strfmt"
	"github.com/go-swagger/go-swagger/validate"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/raphael/goa/design"
	. "github.com/raphael/goa/design/dsl"
	"github.com/raphael/goa/goagen/gen_swagger"
)

// validateSwagger validates that the given swagger object represents a valid Swagger spec.
func validateSwagger(swagger *genswagger.Swagger) {
	b, err := json.Marshal(swagger)
	Ω(err).ShouldNot(HaveOccurred())
	doc, err := spec.New(b, "")
	Ω(err).ShouldNot(HaveOccurred())
	err = validate.Spec(doc, strfmt.NewFormats())
	Ω(err).ShouldNot(HaveOccurred())
}

var _ = Describe("New", func() {
	var swagger *genswagger.Swagger
	var newErr error

	BeforeEach(func() {
		swagger = nil
		newErr = nil
		Design = nil
	})

	JustBeforeEach(func() {
		err := RunDSL()
		Ω(err).ShouldNot(HaveOccurred())
		swagger, newErr = genswagger.New(Design)
	})

	Context("with a valid API definition", func() {
		const (
			title        = "title"
			description  = "description"
			terms        = "terms"
			contactEmail = "contactEmail@goa.design"
			contactName  = "contactName"
			contactURL   = "http://contactURL.com"
			license      = "license"
			licenseURL   = "http://licenseURL.com"
			host         = "host"
			basePath     = "/base"
			docDesc      = "doc description"
			docURL       = "http://docURL.com"
		)

		BeforeEach(func() {
			API("test", func() {
				Title(title)
				Description(description)
				TermsOfService(terms)
				Contact(func() {
					Email(contactEmail)
					Name(contactName)
					URL(contactURL)
				})
				License(func() {
					Name(license)
					URL(licenseURL)
				})
				Docs(func() {
					Description(docDesc)
					URL(docURL)
				})
				Host(host)
				BasePath(basePath)
			})
		})

		It("sets all the basic fields", func() {
			Ω(newErr).ShouldNot(HaveOccurred())
			Ω(swagger).Should(Equal(&genswagger.Swagger{
				Swagger: "2.0",
				Info: &genswagger.Info{
					Title:          title,
					Description:    description,
					TermsOfService: terms,
					Contact: &ContactDefinition{
						Name:  contactName,
						Email: contactEmail,
						URL:   contactURL,
					},
					License: &LicenseDefinition{
						Name: license,
						URL:  licenseURL,
					},
					Version: "",
				},
				Host:     host,
				BasePath: basePath,
				Schemes:  []string{"https"},
				Paths:    make(map[string]*genswagger.Path),
				Consumes: []string{"application/json"},
				Produces: []string{"application/json"},
				ExternalDocs: &genswagger.ExternalDocs{
					Description: docDesc,
					URL:         docURL,
				},
			}))
		})

		It("serializes into valid swagger JSON", func() {
			Ω(newErr).ShouldNot(HaveOccurred())
			b, err := json.Marshal(swagger)
			Ω(err).ShouldNot(HaveOccurred())
			doc, err := spec.New(b, "")
			Ω(err).ShouldNot(HaveOccurred())
			err = validate.Spec(doc, strfmt.NewFormats())
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with base params", func() {
			const (
				basePath    = "/s/:strParam/i/:intParam/n/:numParam/b/:boolParam"
				strParam    = "strParam"
				intParam    = "intParam"
				numParam    = "numParam"
				boolParam   = "boolParam"
				queryParam  = "queryParam"
				description = "description"
				intMin      = 1.0
				floatMax    = 2.4
				enum1       = "enum1"
				enum2       = "enum2"
			)

			BeforeEach(func() {
				base := Design.DSL
				Design.DSL = func() {
					base()
					BasePath(basePath)
					BaseParams(func() {
						Param(strParam, String, func() {
							Description(description)
							Format("email")
						})
						Param(intParam, Integer, func() {
							Minimum(intMin)
						})
						Param(numParam, Number, func() {
							Maximum(floatMax)
						})
						Param(boolParam, Boolean)
						Param(queryParam, func() {
							Enum(enum1, enum2)
						})
					})
				}
			})

			It("sets the BasePath and Parameters fields", func() {
				Ω(newErr).ShouldNot(HaveOccurred())
				Ω(swagger.BasePath).Should(Equal(basePath))
				Ω(swagger.Parameters).Should(HaveLen(5))
				Ω(swagger.Parameters[strParam]).ShouldNot(BeNil())
				Ω(swagger.Parameters[strParam].Name).Should(Equal(strParam))
				Ω(swagger.Parameters[strParam].In).Should(Equal("path"))
				Ω(swagger.Parameters[strParam].Description).Should(Equal("description"))
				Ω(swagger.Parameters[strParam].Required).Should(BeTrue())
				Ω(swagger.Parameters[strParam].Type).Should(Equal("string"))
				Ω(swagger.Parameters[strParam].Format).Should(Equal("email"))
				Ω(swagger.Parameters[intParam]).ShouldNot(BeNil())
				Ω(swagger.Parameters[intParam].Name).Should(Equal(intParam))
				Ω(swagger.Parameters[intParam].In).Should(Equal("path"))
				Ω(swagger.Parameters[intParam].Required).Should(BeTrue())
				Ω(swagger.Parameters[intParam].Type).Should(Equal("integer"))
				Ω(swagger.Parameters[intParam].Minimum).Should(Equal(intMin))
				Ω(swagger.Parameters[numParam]).ShouldNot(BeNil())
				Ω(swagger.Parameters[numParam].Name).Should(Equal(numParam))
				Ω(swagger.Parameters[numParam].In).Should(Equal("path"))
				Ω(swagger.Parameters[numParam].Required).Should(BeTrue())
				Ω(swagger.Parameters[numParam].Type).Should(Equal("number"))
				Ω(swagger.Parameters[numParam].Maximum).Should(Equal(floatMax))
				Ω(swagger.Parameters[boolParam]).ShouldNot(BeNil())
				Ω(swagger.Parameters[boolParam].Name).Should(Equal(boolParam))
				Ω(swagger.Parameters[boolParam].In).Should(Equal("path"))
				Ω(swagger.Parameters[boolParam].Required).Should(BeTrue())
				Ω(swagger.Parameters[boolParam].Type).Should(Equal("boolean"))
				Ω(swagger.Parameters[queryParam]).ShouldNot(BeNil())
				Ω(swagger.Parameters[queryParam].Name).Should(Equal(queryParam))
				Ω(swagger.Parameters[queryParam].In).Should(Equal("query"))
				Ω(swagger.Parameters[queryParam].Type).Should(Equal("string"))
				Ω(swagger.Parameters[queryParam].Enum).Should(Equal([]interface{}{enum1, enum2}))
			})

			It("serializes into valid swagger JSON", func() { validateSwagger(swagger) })
		})

		Context("with response templates", func() {
			const okName = "OK"
			const okDesc = "OK description"
			const notFoundName = "NotFound"
			const notFoundDesc = "NotFound description"
			const notFoundMt = "application/json"
			const headerName = "headerName"

			BeforeEach(func() {
				account := MediaType("application/vnd.goa.test.account", func() {
					Description("Account")
					Attributes(func() {
						Attribute("id", Integer)
						Attribute("href", String)
					})
					View("default", func() {
						Attribute("id")
						Attribute("href")
					})
					View("link", func() {
						Attribute("id")
						Attribute("href")
					})
				})
				mt := MediaType("application/vnd.goa.test.bottle", func() {
					Description("A bottle of wine")
					Attributes(func() {
						Attribute("id", Integer, "ID of bottle")
						Attribute("href", String, "API href of bottle")
						Attribute("account", account, "Owner account")
						Links(func() {
							Link("account") // Defines a link to the Account media type
						})
						Required("id", "href")
					})
					View("default", func() {
						Attribute("id")
						Attribute("href")
						Attribute("links") // Default view renders links
					})
					View("extended", func() {
						Attribute("id")
						Attribute("href")
						Attribute("account") // Extended view renders account inline
						Attribute("links")   // Extended view also renders links
					})
				})
				base := Design.DSL
				Design.DSL = func() {
					base()
					ResponseTemplate(okName, func() {
						Description(okDesc)
						Status(404)
						Media(mt)
						Headers(func() {
							Header(headerName, func() {
								Format("hostname")
							})
						})
					})
					ResponseTemplate(notFoundName, func() {
						Description(notFoundDesc)
						Status(404)
						Media(notFoundMt)
					})
				}
			})

			It("sets the Responses fields", func() {
				Ω(newErr).ShouldNot(HaveOccurred())
				Ω(swagger.Responses).Should(HaveLen(2))
				Ω(swagger.Responses[notFoundName]).ShouldNot(BeNil())
				Ω(swagger.Responses[notFoundName].Description).Should(Equal(notFoundDesc))
				Ω(swagger.Responses[okName]).ShouldNot(BeNil())
				Ω(swagger.Responses[okName].Description).Should(Equal(okDesc))
			})

			It("serializes into valid swagger JSON", func() { validateSwagger(swagger) })
		})
	})
})
