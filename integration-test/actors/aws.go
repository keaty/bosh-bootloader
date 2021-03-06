package actors

import (
	"os"

	"github.com/cloudfoundry/bosh-bootloader/application"
	"github.com/cloudfoundry/bosh-bootloader/aws"
	"github.com/cloudfoundry/bosh-bootloader/aws/clientmanager"
	"github.com/cloudfoundry/bosh-bootloader/aws/cloudformation"
	"github.com/cloudfoundry/bosh-bootloader/aws/ec2"
	"github.com/cloudfoundry/bosh-bootloader/aws/iam"
	"github.com/cloudfoundry/bosh-bootloader/integration-test"

	. "github.com/onsi/gomega"

	awslib "github.com/aws/aws-sdk-go/aws"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
)

type AWS struct {
	stackManager         cloudformation.StackManager
	certificateDescriber iam.CertificateDescriber
	ec2Client            ec2.Client
	cloudFormationClient cloudformation.Client
}

func NewAWS(configuration integration.Config) AWS {
	awsConfig := aws.Config{
		AccessKeyID:     configuration.AWSAccessKeyID,
		SecretAccessKey: configuration.AWSSecretAccessKey,
		Region:          configuration.AWSRegion,
	}

	clientProvider := &clientmanager.ClientProvider{}
	clientProvider.SetConfig(awsConfig)

	stackManager := cloudformation.NewStackManager(clientProvider, application.NewLogger(os.Stdout))
	certificateDescriber := iam.NewCertificateDescriber(clientProvider)

	return AWS{
		stackManager:         stackManager,
		certificateDescriber: certificateDescriber,
		ec2Client:            clientProvider.GetEC2Client(),
		cloudFormationClient: clientProvider.GetCloudFormationClient(),
	}
}

func (a AWS) StackExists(stackName string) bool {
	_, err := a.stackManager.Describe(stackName)

	if err == cloudformation.StackNotFound {
		return false
	}

	Expect(err).NotTo(HaveOccurred())
	return true
}

func (a AWS) GetPhysicalID(stackName, logicalID string) string {
	physicalID, err := a.stackManager.GetPhysicalIDForResource(stackName, logicalID)
	Expect(err).NotTo(HaveOccurred())
	return physicalID
}

func (a AWS) LoadBalancers(stackName string) map[string]string {
	stack, err := a.stackManager.Describe(stackName)
	Expect(err).NotTo(HaveOccurred())

	loadBalancers := map[string]string{}

	for _, loadBalancer := range []string{"CFRouterLoadBalancer", "CFSSHProxyLoadBalancer", "ConcourseLoadBalancer", "ConcourseLoadBalancerURL"} {
		if stack.Outputs[loadBalancer] != "" {
			loadBalancers[loadBalancer] = stack.Outputs[loadBalancer]
		}
	}

	return loadBalancers
}

func (a AWS) DescribeCertificate(certificateName string) iam.Certificate {
	certificate, err := a.certificateDescriber.Describe(certificateName)
	if err != nil && err != iam.CertificateNotFound {
		Expect(err).NotTo(HaveOccurred())
	}

	return certificate
}

func (a AWS) GetEC2InstanceTags(instanceID string) map[string]string {
	describeInstanceInput := &awsec2.DescribeInstancesInput{
		DryRun: awslib.Bool(false),
		Filters: []*awsec2.Filter{
			{
				Name: awslib.String("instance-id"),
				Values: []*string{
					awslib.String(instanceID),
				},
			},
		},
		InstanceIds: []*string{
			awslib.String(instanceID),
		},
	}
	describeInstancesOutput, err := a.ec2Client.DescribeInstances(describeInstanceInput)
	Expect(err).NotTo(HaveOccurred())
	Expect(describeInstancesOutput.Reservations).To(HaveLen(1))
	Expect(describeInstancesOutput.Reservations[0].Instances).To(HaveLen(1))

	instance := describeInstancesOutput.Reservations[0].Instances[0]

	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		tags[awslib.StringValue(tag.Key)] = awslib.StringValue(tag.Value)
	}
	return tags
}

func (a AWS) DescribeKeyPairs(keypairName string) []*awsec2.KeyPairInfo {
	params := &awsec2.DescribeKeyPairsInput{
		Filters: []*awsec2.Filter{{}},
		KeyNames: []*string{
			awslib.String(keypairName),
		},
	}

	keypairOutput, err := a.ec2Client.DescribeKeyPairs(params)
	Expect(err).NotTo(HaveOccurred())

	return keypairOutput.KeyPairs
}
